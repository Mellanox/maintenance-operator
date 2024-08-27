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

package controller

import (
	"context"
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	maintenancev1 "github.com/Mellanox/maintenance-operator/api/v1alpha1"
	operatorlog "github.com/Mellanox/maintenance-operator/internal/log"
	"github.com/Mellanox/maintenance-operator/internal/vars"
)

const (
	defaultMaintenanceOperatorConifgName = "default"
)

// MaintenanceOperatorConfigReconciler reconciles a MaintenanceOperatorConfig object
type MaintenanceOperatorConfigReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	SchedulerReconcierOptions *NodeMaintenanceSchedulerReconcilerOptions
	GarbageCollectorOptions   *GarbageCollectorOptions
}

//+kubebuilder:rbac:groups=maintenance.nvidia.com,resources=maintenanceoperatorconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=maintenance.nvidia.com,resources=maintenanceoperatorconfigs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=maintenance.nvidia.com,resources=maintenanceoperatorconfigs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (r *MaintenanceOperatorConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLog := log.FromContext(ctx)
	reqLog.Info("got request", "name", req.NamespacedName)
	if req.Name != defaultMaintenanceOperatorConifgName || req.Namespace != vars.OperatorNamespace {
		reqLog.Info("request for non default MaintenanceOperatorConfig, ignoring")
		return ctrl.Result{}, nil
	}

	cfg := &maintenancev1.MaintenanceOperatorConfig{}
	err := r.Client.Get(ctx, req.NamespacedName, cfg)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// handle reconcilers options
	reqLog.Info("store scheduler reconciler options", "MaxUnavailable", cfg.Spec.MaxUnavailable,
		"MaxParallelOperations", cfg.Spec.MaxParallelOperations)
	r.SchedulerReconcierOptions.Store(cfg.Spec.MaxUnavailable, cfg.Spec.MaxParallelOperations)
	reqLog.Info("store nodeMaintenance reconciler options", "MaxNodeMaintenanceTimeSeconds", cfg.Spec.MaxNodeMaintenanceTimeSeconds)
	r.GarbageCollectorOptions.Store(time.Second * time.Duration(cfg.Spec.MaxNodeMaintenanceTimeSeconds))

	// handle log level
	reqLog.Info("setting operator log level", "LogLevel", cfg.Spec.LogLevel)
	err = operatorlog.SetLogLevel(string(cfg.Spec.LogLevel))
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MaintenanceOperatorConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&maintenancev1.MaintenanceOperatorConfig{}).
		Complete(r)
}
