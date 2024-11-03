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
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	maintenancev1 "github.com/Mellanox/maintenance-operator/api/v1alpha1"
	"github.com/Mellanox/maintenance-operator/internal/k8sutils"

	e2eutils "github.com/Mellanox/maintenance-operator/test/utils"
)

var (
	ParallelOperationsForTest int32 = 2
)

var _ = Describe("E2E tests of maintenance operator", Ordered, func() {
	var cleaupObjects []client.Object

	BeforeAll(func() {
		By("set maintenanceOperatorConfig maxParallelOperations to 2")
		m := &maintenancev1.MaintenanceOperatorConfig{}
		m.Name = "default"
		m.Namespace = maintenanceOperatorNamespace
		Expect(k8sClient.Get(testContext, client.ObjectKeyFromObject(m), m)).To(Succeed())
		m.Spec.MaxParallelOperations = ptr.To(intstr.FromInt32(ParallelOperationsForTest))
		Expect(k8sClient.Update(testContext, m)).To(Succeed())
		By("deploy test workload")
		uns, err := e2eutils.LoadObjectFromFile("test_workload.yaml")
		Expect(err).NotTo(HaveOccurred())
		deployment := &appsv1.Deployment{}
		Expect(e2eutils.ToConcrete(uns, deployment)).To(Succeed())
		deployment.Spec.Replicas = ptr.To(int32(len(testCluster.workerNodes)))
		Expect(k8sClient.Create(testContext, deployment)).To(Succeed())
		cleaupObjects = append(cleaupObjects, deployment)
		By("wait for deployment pods to be running")
		Eventually(func() int {
			readyPods := 0
			pods := &corev1.PodList{}
			Expect(k8sClient.List(
				testContext, pods, client.InNamespace(deployment.Namespace),
				client.MatchingLabels(deployment.Spec.Selector.MatchLabels))).To(Succeed())
			for _, p := range pods.Items {
				if p.Status.Phase == corev1.PodRunning {
					readyPods++
				}
			}
			return readyPods
		}).WithTimeout(30 * time.Second).Should(Equal(len(testCluster.workerNodes)))
	})

	AfterAll(func() {
		By("collecting cluster artifacts")
		err := clusterCollector.Run(testContext)
		if err != nil {
			GinkgoLogr.Error(err, "failed to collect cluster artifacts")
		}

		for _, o := range cleaupObjects {
			err := k8sClient.Delete(testContext, o)
			if err != nil && !k8serrors.IsNotFound(err) {
				GinkgoLogr.Error(err, "failed to delete object", "name", o.GetName())
			}
		}

		// wait for objects to be deleted
		for _, o := range cleaupObjects {
			Eventually(func() error {
				return k8sClient.Get(testContext, client.ObjectKeyFromObject(o), o)
			}, time.Minute, time.Second).ShouldNot(Succeed())
		}
	})

	It("E2E test flow should run successfully", func() {
		nmUns, err := e2eutils.LoadObjectFromFile("test_maintenance.yaml")
		Expect(err).NotTo(HaveOccurred())
		nmTpl := &maintenancev1.NodeMaintenance{}
		Expect(e2eutils.ToConcrete(nmUns, nmTpl)).To(Succeed())
		By("create NodeMaintenance requests for all worker nodes with two requests per node with different requestorIDs")
		for _, nodeName := range testCluster.workerNodes {
			nm1 := generateNMFromTemplate(nmTpl, fmt.Sprintf("%s-%s", "one", nodeName), nodeName, "one.test.com")
			nm2 := generateNMFromTemplate(nmTpl, fmt.Sprintf("%s-%s", "two", nodeName), nodeName, "two.test.com")
			Expect(k8sClient.Create(testContext, nm1)).To(Succeed())
			cleaupObjects = append(cleaupObjects, nm1)
			Expect(k8sClient.Create(testContext, nm2)).To(Succeed())
			cleaupObjects = append(cleaupObjects, nm2)
		}

		By("validate NoteMaintenance state in the cluster")
		for i := 0; i < len(testCluster.workerNodes)*2; {
			readyNMs := validateState(int(ParallelOperationsForTest))
			By("delete ready NodeMaintenance requests")
			for _, nm := range readyNMs {
				Expect(k8sClient.Delete(testContext, nm)).To(Succeed())
			}
			By("wait for ready NodeMaintenance to be deleted")
			for _, nm := range readyNMs {
				Eventually(func() error {
					return k8sClient.Get(testContext, client.ObjectKeyFromObject(nm), nm)
				}).WithTimeout(60 * time.Second).ShouldNot(Succeed())
			}
			i += len(readyNMs)
		}
		By("ensure all NodeMaintenance requests are deleted")
		Eventually(func() int {
			nms := &maintenancev1.NodeMaintenanceList{}
			Expect(k8sClient.List(testContext, nms)).To(Succeed())
			return len(nms.Items)
		}).WithTimeout(60 * time.Second).Should(Equal(0))
	})
})

// generateNMFromTemplate generates a new NodeMaintenance object from the template with the provided name, nodeName and requestorID
func generateNMFromTemplate(tpl *maintenancev1.NodeMaintenance, name, nodeName, requestorID string) *maintenancev1.NodeMaintenance {
	nm := tpl.DeepCopy()
	nm.Name = name
	nm.Spec.NodeName = nodeName
	nm.Spec.RequestorID = requestorID
	return nm
}

// validateState validates that we eventually have expectedReady NodeMaintenances in ready state and then ensure
// it stays that way consistently. return ready NodeMaintenances for further processing.
func validateState(expectedReady int) []*maintenancev1.NodeMaintenance {
	By("wait for NodeMaintenance requests to be in ready state")
	Eventually(getReadyNodeMaintenance).WithTimeout(60 * time.Second).WithPolling(1 * time.Second).
		Should(HaveLen(expectedReady))
	Consistently(getReadyNodeMaintenance).WithTimeout(10 * time.Second).WithPolling(1 * time.Second).
		Should(HaveLen(expectedReady))
	readyNMs := getReadyNodeMaintenance()
	By("ensure ready NodeMaintenance requests are for different nodes")
	nodeNames := make(map[string]struct{})
	for _, nm := range readyNMs {
		if _, ok := nodeNames[nm.Spec.NodeName]; ok {
			Fail(fmt.Sprintf("NodeMaintenance request for node %s is duplicated", nm.Spec.NodeName))
		}
		nodeNames[nm.Spec.NodeName] = struct{}{}
	}
	return readyNMs
}

// getReadyNodeMaintenance returns NodeMaintenance objects in Ready state
func getReadyNodeMaintenance() []*maintenancev1.NodeMaintenance {
	nms := &maintenancev1.NodeMaintenanceList{}
	ExpectWithOffset(1, k8sClient.List(testContext, nms)).To(Succeed())
	var readyNMs []*maintenancev1.NodeMaintenance
	for i := range nms.Items {
		if k8sutils.GetReadyConditionReason(&nms.Items[i]) == maintenancev1.ConditionReasonReady {
			readyNMs = append(readyNMs, &nms.Items[i])
		}
	}
	GinkgoWriter.Printf("Ready NodeMaintenances: %d\n", len(readyNMs))
	return readyNMs
}
