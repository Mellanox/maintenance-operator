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

package e2e

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	maintenancev1 "github.com/Mellanox/maintenance-operator/api/v1alpha1"
	"github.com/Mellanox/maintenance-operator/test/utils"
)

type testClusterInfo struct {
	workerNodes       []string
	controlPlaneNodes []string
}

// discoverNodes discovers control plane and worker nodes in the cluster.
// then stores them in testClusterInfo
func (tc *testClusterInfo) discoverNodes(k8sClient client.Client) error {
	nodes := &corev1.NodeList{}
	if err := k8sClient.List(testContext, nodes); err != nil {
		return errors.Wrap(err, "failed to list nodes")
	}

	for _, node := range nodes.Items {
		if _, ok := node.Labels["node-role.kubernetes.io/control-plane"]; ok {
			tc.controlPlaneNodes = append(tc.controlPlaneNodes, node.Name)
		}
		if _, ok := node.Labels["node-role.kubernetes.io/worker"]; ok {
			tc.workerNodes = append(tc.workerNodes, node.Name)
		}
	}

	if len(tc.controlPlaneNodes) == 0 {
		return fmt.Errorf("no control plane nodes found")
	}

	if len(tc.workerNodes) == 0 {
		return fmt.Errorf("no worker nodes found")
	}

	return nil
}

var (
	// testContext is a context for testing.
	testContext context.Context
	// k8sClient is a Kubernetes client.
	k8sClient client.Client
	// testKubeconfig is the path to the kubeconfig file used for testing.
	testKubeconfig string
	// testCluster contains cluster information
	testCluster testClusterInfo
	// maintenanceOperatorNamespace is the namespace where the maintenance operator is installed.
	maintenanceOperatorNamespace string
	// artifactsDir is the directory where artifacts are stored.
	artifcatsDir string
	// clusterCollector is used to collect cluster resources.
	clusterCollector *utils.ClusterCollector
)

func init() {
	flag.StringVar(&testKubeconfig, "e2e.kubeconfig", "", "path to the kubeconfig file used for testing")
	flag.StringVar(&maintenanceOperatorNamespace, "e2e.maintenanceOperatorNamespace", "maintenance-operator", "namespace where the maintenance operator is installed")
	flag.StringVar(&artifcatsDir, "e2e.artifactsDir", "../../artifacts", "path to the directory where artifacts will be stored")
}

// Run e2e tests using the Ginkgo runner.
func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	fmt.Fprintf(GinkgoWriter, "Starting maintenance-operator suite\n")
	RunSpecs(t, "e2e suite")
}

var _ = BeforeSuite(func() {
	fmt.Fprintf(GinkgoWriter, "BeforeSuite\n")
	// set up context
	testContext = ctrl.SetupSignalHandler()

	// set up logger
	ctrl.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	// setup scheme
	fmt.Fprintf(GinkgoWriter, "setup scheme\n")
	s := runtime.NewScheme()
	Expect(scheme.AddToScheme(s)).To(Succeed())
	Expect(maintenancev1.AddToScheme(s)).To(Succeed())

	// if testKubeconfig is not set, default it to $HOME/.kube/config
	fmt.Fprintf(GinkgoWriter, "get kubeconfig\n")
	home, exists := os.LookupEnv("HOME")
	Expect(exists).To(BeTrue())
	if testKubeconfig == "" {
		testKubeconfig = filepath.Join(home, ".kube/config")
	}

	// create k8sClient
	fmt.Fprintf(GinkgoWriter, "create client\n")
	restConfig, err := clientcmd.BuildConfigFromFlags("", testKubeconfig)
	Expect(err).NotTo(HaveOccurred())
	k8sClient, err = client.New(restConfig, client.Options{Scheme: s})
	Expect(err).NotTo(HaveOccurred())
	kubeClient, err := kubernetes.NewForConfig(restConfig)
	Expect(err).NotTo(HaveOccurred())

	// create clusterCollector
	fmt.Fprintf(GinkgoWriter, "create cluster collector\n")
	clusterCollector = utils.NewClusterCollector(k8sClient, kubeClient, artifcatsDir)

	// discover nodes
	fmt.Fprintf(GinkgoWriter, "discover nodes\n")
	Expect(testCluster.discoverNodes(k8sClient)).To(Succeed())

	fmt.Fprintf(GinkgoWriter, "Cluster Information\n")
	fmt.Fprintf(GinkgoWriter, "ControlPlane Nodes: %+v\n", testCluster.controlPlaneNodes)
	fmt.Fprintf(GinkgoWriter, "Worker Nodes: %+v\n", testCluster.workerNodes)
	fmt.Fprintf(GinkgoWriter, "BeforeSuite End\n")
})
