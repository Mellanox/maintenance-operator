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
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kptr "k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlconfig "sigs.k8s.io/controller-runtime/pkg/config"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	maintenancev1 "github.com/Mellanox/maintenance-operator/api/v1alpha1"
	"github.com/Mellanox/maintenance-operator/internal/cordon"
	"github.com/Mellanox/maintenance-operator/internal/drain"
	"github.com/Mellanox/maintenance-operator/internal/k8sutils"
	"github.com/Mellanox/maintenance-operator/internal/podcompletion"
	"github.com/Mellanox/maintenance-operator/internal/testutils"
)

var _ = Describe("NodeMaintenance Controller", func() {
	Context("Envtests", func() {
		var nmObjectsToCleanup []*maintenancev1.NodeMaintenance
		var nodeObjectsToCleanup []*corev1.Node
		var podObjectsToCleanup []*corev1.Pod

		var reconciler *NodeMaintenanceReconciler

		// test context, TODO(adrianc): use ginkgo spec context
		var testCtx context.Context

		BeforeEach(func() {
			testCtx = context.Background()
			// create node objects for scheduler to use
			By("create test nodes")
			nodes := testutils.GetTestNodes("test-node", 1, false)
			for i := range nodes {
				Expect(k8sClient.Create(testCtx, nodes[i])).ToNot(HaveOccurred())
				nodeObjectsToCleanup = append(nodeObjectsToCleanup, nodes[i])
			}

			// create controller manager
			By("create controller manager")
			mgr, err := ctrl.NewManager(cfg, ctrl.Options{
				Scheme:     k8sClient.Scheme(),
				Metrics:    metricsserver.Options{BindAddress: "0"},
				Controller: ctrlconfig.Controller{SkipNameValidation: kptr.To(true)},
			})
			Expect(err).ToNot(HaveOccurred())

			// create reconciler
			By("create NodeMaintenanceReconciler")
			reconciler = &NodeMaintenanceReconciler{
				Client:                   k8sClient,
				Scheme:                   k8sClient.Scheme(),
				CordonHandler:            cordon.NewCordonHandler(k8sClient, k8sInterface),
				WaitPodCompletionHandler: podcompletion.NewPodCompletionHandler(k8sClient),
				DrainManager: drain.NewManager(ctrllog.Log.WithName("DrainManager"),
					testCtx, k8sInterface),
			}

			// setup reconciler with manager
			By("setup NodeMaintenanceReconciler with controller manager")
			Expect(reconciler.SetupWithManager(testCtx, mgr, ctrllog.Log.WithName("NodeMaintenanceReconciler"))).
				ToNot(HaveOccurred())

			// start manager
			testMgrCtx, cancel := context.WithCancel(testCtx)
			By("start manager")
			wg := sync.WaitGroup{}
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer GinkgoRecover()
				By("Start controller manager")
				err := mgr.Start(testMgrCtx)
				Expect(err).ToNot(HaveOccurred())
			}()

			DeferCleanup(func() {
				By("Shut down controller manager")
				cancel()
				wg.Wait()
			})
		})

		AfterEach(func() {
			var allObj []client.Object
			By("Cleanup NodeMaintenance resources")
			for _, nm := range nmObjectsToCleanup {
				// remove finalizers if any and delete object
				err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(nm), nm)
				if err == nil {
					// delete obj
					err = k8sClient.Delete(testCtx, nm)
					if err != nil && k8serrors.IsNotFound(err) {
						err = nil
					}
					Expect(err).ToNot(HaveOccurred())
					// remove finalizers
					if len(nm.GetFinalizers()) > 0 {
						nm.SetFinalizers(nil)
						err := k8sClient.Update(testCtx, nm)
						if err != nil && k8serrors.IsNotFound(err) {
							err = nil
						}
						Expect(err).ToNot(HaveOccurred())
					}
				}
				allObj = append(allObj, nm)
			}
			nmObjectsToCleanup = make([]*maintenancev1.NodeMaintenance, 0)

			By("Cleanup Node resources")
			for _, n := range nodeObjectsToCleanup {
				Expect(k8sClient.Delete(testCtx, n)).To(Succeed())
				allObj = append(allObj, n)
			}
			nodeObjectsToCleanup = make([]*corev1.Node, 0)

			By("Cleanup Pod resources")
			for _, p := range podObjectsToCleanup {
				var grace int64
				err := k8sClient.Delete(testCtx, p, &client.DeleteOptions{GracePeriodSeconds: &grace, Preconditions: &metav1.Preconditions{UID: &p.UID}})
				if err != nil && k8serrors.IsNotFound(err) {
					err = nil
				}
				Expect(err).ToNot(HaveOccurred())
				allObj = append(allObj, p)
			}
			podObjectsToCleanup = make([]*corev1.Pod, 0)

			By("Wait for all objects to be removed from api")
			for _, obj := range allObj {
				Eventually(func() bool {
					err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(obj), obj)
					return k8serrors.IsNotFound(err)
				}).WithTimeout(30*time.Second).WithPolling(1*time.Second).Should(BeTrue(), "Object not deleted", obj.GetNamespace(), obj.GetName())
			}
		})

		It("Should transition new NodeMaintenance Resource to Pending", func() {
			nm := testutils.GetTestNodeMaintenance("test-nm", "test-node-0", "some-operator.nvidia.com", "")
			Expect(k8sClient.Create(testCtx, nm)).ToNot(HaveOccurred())
			nmObjectsToCleanup = append(nmObjectsToCleanup, nm)

			By("Eventually NodeMaintenance condition is set to Pending")
			Eventually(testutils.GetReadyConditionReasonForFn(testCtx, k8sClient, client.ObjectKeyFromObject(nm))).
				WithTimeout(10 * time.Second).WithPolling(1 * time.Second).Should(Equal(maintenancev1.ConditionReasonPending))

			By("ConditionChanged event with Pending msg is sent for NodeMaintenance")
			Eventually(testutils.EventsForObjFn(testCtx, k8sClient, nm.UID)).WithTimeout(10 * time.Second).
				WithPolling(1 * time.Second).Should(Equal([]string{maintenancev1.ConditionReasonPending}))
		})

		It("Full lifecycle of NodeMaintenance", func() {
			By("Create NodeMaintenance")
			nm := testutils.GetTestNodeMaintenance("test-nm", "test-node-0", "some-operator.nvidia.com", "")
			nm.Spec.WaitForPodCompletion = &maintenancev1.WaitForPodCompletionSpec{PodSelector: "for=wait-completion"}
			nm.Spec.DrainSpec = &maintenancev1.DrainSpec{
				Force:       true,
				PodSelector: "for=drain",
			}
			Expect(k8sClient.Create(testCtx, nm)).ToNot(HaveOccurred())
			nmObjectsToCleanup = append(nmObjectsToCleanup, nm)

			By("Create test pods")
			// pod to wait on for completion
			podForWaitCompletion := testutils.GetTestPod("test-pod", "test-node-0", map[string]string{"for": "wait-completion"})
			Expect(k8sClient.Create(testCtx, podForWaitCompletion)).ToNot(HaveOccurred())
			podObjectsToCleanup = append(podObjectsToCleanup, podForWaitCompletion)
			// pod for drain
			podForDrain := testutils.GetTestPod("test-pod-2", "test-node-0", map[string]string{"for": "drain"})
			Expect(k8sClient.Create(testCtx, podForDrain)).ToNot(HaveOccurred())
			podObjectsToCleanup = append(podObjectsToCleanup, podForDrain)

			By("Eventually NodeMaintenance condition is set to Pending")
			Eventually(testutils.GetReadyConditionReasonForFn(testCtx, k8sClient, client.ObjectKeyFromObject(nm))).
				WithTimeout(10 * time.Second).WithPolling(1 * time.Second).Should(Equal(maintenancev1.ConditionReasonPending))

			By("Set NodeMaintenance to Scheduled")
			Expect(k8sClient.Get(testCtx, types.NamespacedName{Namespace: "default", Name: "test-nm"}, nm)).ToNot(HaveOccurred())
			Expect(k8sutils.SetReadyConditionReason(testCtx, k8sClient, nm, maintenancev1.ConditionReasonScheduled)).ToNot(HaveOccurred())

			By("Eventually NodeMaintenance condition is set to WaitForPodCompletion")
			Eventually(testutils.GetReadyConditionReasonForFn(testCtx, k8sClient, client.ObjectKeyFromObject(nm))).
				WithTimeout(10 * time.Second).WithPolling(1 * time.Second).Should(Equal(maintenancev1.ConditionReasonWaitForPodCompletion))

			By("Consistently NodeMaintenance remains in WaitForPodComletion")
			Consistently(testutils.GetReadyConditionReasonForFn(testCtx, k8sClient, client.ObjectKeyFromObject(nm))).
				Within(time.Second).WithPolling(100 * time.Millisecond).
				Should(Equal(maintenancev1.ConditionReasonWaitForPodCompletion))

			By("Setting Additional requestor for node maintenance")
			Expect(k8sClient.Get(testCtx, types.NamespacedName{Namespace: "default", Name: "test-nm"}, nm)).ToNot(HaveOccurred())
			nm.Spec.AdditionalRequestors = append(nm.Spec.AdditionalRequestors, "another-requestor.nvidia.com")
			Expect(k8sClient.Update(testCtx, nm)).ToNot(HaveOccurred())

			By("After deleting wait for completion pod, NodeMaintenance is Draining")
			// NOTE(adrianc): for pods we must provide DeleteOptions as below else apiserver will not delete pod object
			var grace int64
			Expect(k8sClient.Delete(testCtx, podForWaitCompletion, &client.DeleteOptions{
				GracePeriodSeconds: &grace, Preconditions: &metav1.Preconditions{UID: &podForWaitCompletion.UID}})).
				ToNot(HaveOccurred())
			Eventually(testutils.GetReadyConditionReasonForFn(testCtx, k8sClient, client.ObjectKeyFromObject(nm))).
				WithTimeout(20 * time.Second).WithPolling(1 * time.Second).Should(Equal(maintenancev1.ConditionReasonDraining))

			By("Eventually NodeMaintenance is Ready")
			// NOTE(adrianc): as above comment, we need to "help" drain to delete the targeted pod since api server will not
			// delete pod object without setting specific delete options
			Expect(k8sClient.Delete(testCtx, podForDrain, &client.DeleteOptions{
				GracePeriodSeconds: &grace, Preconditions: &metav1.Preconditions{UID: &podForDrain.UID}})).
				ToNot(HaveOccurred())
			Eventually(testutils.GetReadyConditionReasonForFn(testCtx, k8sClient, client.ObjectKeyFromObject(nm))).
				WithTimeout(20 * time.Second).WithPolling(5 * time.Second).Should(Equal(maintenancev1.ConditionReasonReady))

			By("Validating expected")
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node-0",
				},
			}
			Expect(k8sClient.Get(testCtx, client.ObjectKeyFromObject(node), node)).ToNot(HaveOccurred())
			Expect(node.Spec.Unschedulable).To(BeTrue())

			Expect(k8sClient.Get(testCtx, client.ObjectKeyFromObject(nm), nm)).ToNot(HaveOccurred())
			Expect(nm.Annotations[cordon.NodeInitialStateUnschedulableAnnot]).To(Equal("false"))
			Expect(nm.Annotations[podcompletion.WaitForPodCompletionStartAnnot]).ToNot(BeEmpty())
			Expect(nm.Annotations[ReadyTimeAnnotation]).ToNot(BeEmpty())

			By("ConditionChanged events are sent for NodeMaintenance")
			Eventually(testutils.EventsForObjFn(testCtx, k8sClient, nm.UID)).WithTimeout(10 * time.Second).
				WithPolling(1 * time.Second).Should(ContainElements(
				maintenancev1.ConditionReasonPending, maintenancev1.ConditionReasonCordon,
				maintenancev1.ConditionReasonWaitForPodCompletion, maintenancev1.ConditionReasonDraining,
				maintenancev1.ConditionReasonReady))

			By("Object is not removed when deleted due to having AdditionalRequestors")
			Expect(k8sClient.Delete(testCtx, nm)).ToNot(HaveOccurred())
			Consistently(k8sClient.Get(testCtx, client.ObjectKeyFromObject(nm), nm)).
				Within(time.Second).WithPolling(100 * time.Millisecond).
				Should(Succeed())

			By("Node remains cordoned")
			Expect(k8sClient.Get(testCtx, client.ObjectKeyFromObject(node), node)).ToNot(HaveOccurred())
			Expect(node.Spec.Unschedulable).To(BeTrue())

			By("Remove AdditionalRequestros")
			nm.Spec.AdditionalRequestors = nil
			Expect(k8sClient.Update(testCtx, nm)).ToNot(HaveOccurred())

			By("Should Uncordon node after NodeMaintenance is deleted")
			Eventually(func() bool {
				node := &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-node-0",
					},
				}
				Expect(k8sClient.Get(testCtx, client.ObjectKeyFromObject(node), node)).ToNot(HaveOccurred())
				return node.Spec.Unschedulable
			}).WithTimeout(10 * time.Second).WithPolling(1 * time.Second).Should(BeFalse())

			By("Node maintenance no longer exists")
			err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(nm), nm)
			Expect(err).To(HaveOccurred())
			Expect(k8serrors.IsNotFound(err)).To(BeTrue())
		})
	})
})
