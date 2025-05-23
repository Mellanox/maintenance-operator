/*
 Copyright 2024, NVIDIA CORPORATION & AFFILIATES

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"os"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	ocpconfigv1 "github.com/openshift/api/config/v1"
	mcv1 "github.com/openshift/api/machineconfiguration/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	maintenancev1alpha1 "github.com/Mellanox/maintenance-operator/api/v1alpha1"
	"github.com/Mellanox/maintenance-operator/internal/controller"
	"github.com/Mellanox/maintenance-operator/internal/cordon"
	"github.com/Mellanox/maintenance-operator/internal/drain"
	operatorlog "github.com/Mellanox/maintenance-operator/internal/log"
	"github.com/Mellanox/maintenance-operator/internal/openshift"
	"github.com/Mellanox/maintenance-operator/internal/podcompletion"
	"github.com/Mellanox/maintenance-operator/internal/scheduler"
	"github.com/Mellanox/maintenance-operator/internal/version"
	maintenancewebhook "github.com/Mellanox/maintenance-operator/internal/webhook"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(ocpconfigv1.Install(scheme))
	utilruntime.Must(mcv1.Install(scheme))

	utilruntime.Must(maintenancev1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

//nolint:funlen
func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var secureMetrics bool
	var enableHTTP2 bool
	var printVersion bool
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", false,
		"If set the metrics endpoint is served securely")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	flag.BoolVar(&printVersion, "version", false, "print version and exit")

	operatorlog.BindFlags(flag.CommandLine)
	flag.Parse()

	if printVersion {
		fmt.Printf("%s\n", version.GetVersionString())
		os.Exit(0)
	}
	operatorlog.InitLog()

	setupLog.Info("Maintenance Operator", "version", version.GetVersionString())

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancelation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	tlsOpts := []func(*tls.Config){}
	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	webhookServer := webhook.NewServer(webhook.Options{
		TLSOpts: tlsOpts,
	})

	restConfig := ctrl.GetConfigOrDie()
	mgr, err := ctrl.NewManager(restConfig, ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress:   metricsAddr,
			SecureServing: secureMetrics,
			TLSOpts:       tlsOpts,
		},
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "a9620a8f.nvidia.com",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to create manager")
		os.Exit(1)
	}

	k8sInterface, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		setupLog.Error(err, "unable to create kubernetes interface")
		os.Exit(1)
	}

	ctx := ctrl.SetupSignalHandler()
	mgrClient := mgr.GetClient()

	ocpUtils, err := openshift.NewOpenshiftUtils(ctx, mgr.GetAPIReader())
	if err != nil {
		setupLog.Error(err, "unable to create openshift utils")
		os.Exit(1)
	}

	if ocpUtils.IsOpenshift() {
		setupLog.Info("openshift cluster detected",
			"isOpenshift", ocpUtils.IsOpenshift(), "isHypershift", ocpUtils.IsHypershift())
	}

	nmReconciler := &controller.NodeMaintenanceReconciler{
		Client:                   mgrClient,
		Scheme:                   mgr.GetScheme(),
		CordonHandler:            cordon.NewCordonHandler(mgrClient, k8sInterface),
		WaitPodCompletionHandler: podcompletion.NewPodCompletionHandler(mgrClient),
		DrainManager:             drain.NewManager(ctrl.Log.WithName("DrainManager"), ctx, k8sInterface),
		MCPManager:               nil,
	}

	if ocpUtils.IsOpenshift() && !ocpUtils.IsHypershift() {
		nmReconciler.MCPManager = openshift.NewMCPManager(mgrClient)
	}

	if err = nmReconciler.SetupWithManager(ctx, mgr, ctrl.Log.WithName("NodeMaintenanceReconciler")); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "NodeMaintenance")
		os.Exit(1)
	}

	gcOptions := controller.NewGarbageCollectorOptions()
	gcLog := ctrl.Log.WithName("NodeMaintenanceGarbageCollector")
	if err = controller.NewNodeMaintenanceGarbageCollector(
		mgrClient, gcOptions, gcLog).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "NodeMaintenanceGarbageCollector")
		os.Exit(1)
	}

	nmsrOptions := controller.NewNodeMaintenanceSchedulerReconcilerOptions()
	nmsrLog := ctrl.Log.WithName("NodeMaintenanceScheduler")
	if err = (&controller.NodeMaintenanceSchedulerReconciler{
		Client:  mgrClient,
		Scheme:  mgr.GetScheme(),
		Options: nmsrOptions,
		Log:     nmsrLog,
		Sched:   scheduler.NewDefaultScheduler(nmsrLog.WithName("DefaultScheduler")),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "NodeMaintenanceScheduler")
		os.Exit(1)
	}

	if err = (&controller.MaintenanceOperatorConfigReconciler{
		Client:                    mgrClient,
		Scheme:                    mgr.GetScheme(),
		GarbageCollectorOptions:   gcOptions,
		SchedulerReconcierOptions: nmsrOptions,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "MaintenanceOperatorConfig")
		os.Exit(1)
	}

	if os.Getenv("ENABLE_WEBHOOKS") != "false" {
		if err = maintenancewebhook.NewNodeMaintenanceWebhook(mgrClient).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "NodeMaintenanceWebhook")
			os.Exit(1)
		}
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
