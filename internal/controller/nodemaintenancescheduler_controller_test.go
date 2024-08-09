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
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"k8s.io/apimachinery/pkg/types"

	maintenancev1 "github.com/Mellanox/maintenance-operator/api/v1alpha1"
	"github.com/Mellanox/maintenance-operator/internal/k8sutils"
	"github.com/Mellanox/maintenance-operator/internal/scheduler"
	"github.com/Mellanox/maintenance-operator/internal/testutils"
)

var _ = Describe("NodeMaintenanceScheduler Controller", func() {
	Context("Envtests", func() {
		var nmObjectsToCleanup []*maintenancev1.NodeMaintenance
		var nodeObjectsToCleanup []*corev1.Node

		var reconciler *NodeMaintenanceSchedulerReconciler
		var mgr manager.Manager

		// test context, TODO(adrianc): use ginkgo spec context
		var testCtx context.Context

		BeforeEach(func() {
			testCtx = context.Background()
			// create node objects for scheduler to use
			By("create test nodes")
			nodes := testutils.GetTestNodes("test-node", 5, false)
			for i := range nodes {
				Expect(k8sClient.Create(testCtx, nodes[i])).ToNot(HaveOccurred())
				nodeObjectsToCleanup = append(nodeObjectsToCleanup, nodes[i])
			}

			// create controller manager
			By("create controller manager")
			var err error
			mgr, err = ctrl.NewManager(cfg, ctrl.Options{
				Scheme:  k8sClient.Scheme(),
				Metrics: server.Options{BindAddress: "0"},
			})
			Expect(err).ToNot(HaveOccurred())

			// create reconciler
			By("create NodeMaintenanceSchedulerReconciler")
			nmSchedulerReconcilerLog := ctrllog.Log.WithName("NodeMaintenanceScheduler")
			reconciler = &NodeMaintenanceSchedulerReconciler{
				Client:  k8sClient,
				Scheme:  k8sClient.Scheme(),
				Log:     nmSchedulerReconcilerLog,
				Sched:   scheduler.NewDefaultScheduler(nmSchedulerReconcilerLog.WithName("DefaultScheduler")),
				Options: NewNodeMaintenanceSchedulerReconcilerOptions(),
			}

			// setup reconciler with manager
			By("setup NodeMaintenanceSchedulerReconciler with controller manager")
			Expect(reconciler.SetupWithManager(mgr)).ToNot(HaveOccurred())

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
			By("Cleanup NodeMaintenance resources")
			for _, nm := range nmObjectsToCleanup {
				Expect(k8sClient.Delete(testCtx, nm)).To(Succeed())
			}
			nmObjectsToCleanup = make([]*maintenancev1.NodeMaintenance, 0)

			By("Cleanup Node resources")
			for _, n := range nodeObjectsToCleanup {
				Expect(k8sClient.Delete(testCtx, n)).To(Succeed())
			}
			nodeObjectsToCleanup = make([]*corev1.Node, 0)
		})

		It("should successfully reconcile a single NodeMaintenance resource", func() {
			By("creating the custom resource for the Kind NodeMaintenance")
			nodeMaintenanceResource := testutils.GetTestNodeMaintenance("test-nm", "test-node-1", "some-operator.nvidia.com", "")
			Expect(k8sClient.Create(testCtx, nodeMaintenanceResource)).To(Succeed())
			nmObjectsToCleanup = append(nmObjectsToCleanup, nodeMaintenanceResource)

			By("update Ready condition reason to Pending")
			Expect(k8sutils.SetReadyConditionReason(testCtx, k8sClient, nodeMaintenanceResource, maintenancev1.ConditionReasonPending)).
				ToNot(HaveOccurred())

			By("Eventually NodeMaintenance condition is set to Scheduled")
			Eventually(func() string {
				nm := &maintenancev1.NodeMaintenance{}
				err := k8sClient.Get(testCtx, types.NamespacedName{Namespace: "default", Name: "test-nm"}, nm)
				if err == nil {
					return k8sutils.GetReadyConditionReason(nm)
				}
				return ""
			}).WithTimeout(1 * time.Minute).WithPolling(1 * time.Second).Should(Equal(maintenancev1.ConditionReasonScheduled))

			By("ConditionChanged event with Scheduled msg is sent for NodeMaintenance")
			Eventually(func() string {
				el := &corev1.EventList{}
				err := k8sClient.List(testCtx, el, client.MatchingFields{"involvedObject.uid": string(nodeMaintenanceResource.UID)})
				if err == nil && len(el.Items) > 0 {
					return el.Items[0].Message
				}
				return ""
			}).WithTimeout(10 * time.Second).WithPolling(1 * time.Second).Should(Equal(maintenancev1.ConditionReasonScheduled))
		})
	})

	Context("Unit-Tests", func() {
		Context("PreSchedule()", func() {
			var nodes []*corev1.Node
			var maintenances []*maintenancev1.NodeMaintenance
			var reconciler *NodeMaintenanceSchedulerReconciler

			BeforeEach(func() {
				nodes = testutils.GetTestNodes("node", 5, false)
				maintenances = nil
				reconciler = &NodeMaintenanceSchedulerReconciler{
					Log:     ctrllog.Log.WithName("NodeMaintenanceScheduler"),
					Options: NewNodeMaintenanceSchedulerReconcilerOptions(),
				}
			})

			It("single NodeMaintenance", func() {
				maintenances = []*maintenancev1.NodeMaintenance{
					testutils.GetTestNodeMaintenance("test-nm", "node-0", "test-operator.nvidia.com", maintenancev1.ConditionReasonPending),
				}

				schedCtx, err := reconciler.preSchedule(nodes, maintenances)
				Expect(err).ToNot(HaveOccurred())
				validateSchedulerContext(schedCtx, 1, 1, sets.New("node-0"), sets.New("test-nm"))
			})

			It("Multiple node maintenance", func() {
				maintenances = []*maintenancev1.NodeMaintenance{
					// maintenances in progress
					testutils.GetTestNodeMaintenance("test-nm-0", "node-0", "test-operator.nvidia.com", maintenancev1.ConditionReasonScheduled),
					// maintenances pending
					testutils.GetTestNodeMaintenance("test-nm-1", "node-0", "test-operator.nvidia.com", maintenancev1.ConditionReasonPending),
					testutils.GetTestNodeMaintenance("test-nm-2", "node-1", "test-operator.nvidia.com", maintenancev1.ConditionReasonPending),
					testutils.GetTestNodeMaintenance("test-nm-3", "node-1", "test-operator.nvidia.com", maintenancev1.ConditionReasonPending),
					testutils.GetTestNodeMaintenance("test-nm-4", "node-2", "test-operator.nvidia.com", maintenancev1.ConditionReasonPending),
				}
				reconciler.Options.Store(nil, &intstr.IntOrString{Type: intstr.Int, IntVal: 3})
				reconciler.Options.Load()

				schedCtx, err := reconciler.preSchedule(nodes, maintenances)
				Expect(err).ToNot(HaveOccurred())
				validateSchedulerContext(schedCtx, 2, 2, sets.New("node-1", "node-2"), sets.New("test-nm-2", "test-nm-3", "test-nm-4"))
			})

			It("Multiple node maintenance - no available slots", func() {
				maintenances = []*maintenancev1.NodeMaintenance{
					// maintenances in progress
					testutils.GetTestNodeMaintenance("test-nm-0", "node-0", "test-operator.nvidia.com", maintenancev1.ConditionReasonScheduled),
					// maintenances pending
					testutils.GetTestNodeMaintenance("test-nm-1", "node-0", "test-operator.nvidia.com", maintenancev1.ConditionReasonPending),
					testutils.GetTestNodeMaintenance("test-nm-2", "node-1", "test-operator.nvidia.com", maintenancev1.ConditionReasonPending),
				}
				reconciler.Options.Store(nil, &intstr.IntOrString{Type: intstr.Int, IntVal: 1})
				reconciler.Options.Load()

				schedCtx, err := reconciler.preSchedule(nodes, maintenances)
				Expect(err).ToNot(HaveOccurred())
				validateSchedulerContext(schedCtx, 0, 0, sets.New("node-1"), sets.New("test-nm-2"))
			})

			It("Multiple node maintenance - no available nodes", func() {
				maintenances = []*maintenancev1.NodeMaintenance{
					// maintenances in progress
					testutils.GetTestNodeMaintenance("test-nm-0", "node-0", "test-operator.nvidia.com", maintenancev1.ConditionReasonScheduled),
					testutils.GetTestNodeMaintenance("test-nm-1", "node-1", "test-operator.nvidia.com", maintenancev1.ConditionReasonScheduled),
					// maintenances pending
					testutils.GetTestNodeMaintenance("test-nm-2", "node-0", "test-operator.nvidia.com", maintenancev1.ConditionReasonPending),
					testutils.GetTestNodeMaintenance("test-nm-3", "node-1", "test-operator.nvidia.com", maintenancev1.ConditionReasonPending),
				}
				reconciler.Options.Store(nil, &intstr.IntOrString{Type: intstr.Int, IntVal: 3})
				reconciler.Options.Load()

				schedCtx, err := reconciler.preSchedule(nodes, maintenances)
				Expect(err).ToNot(HaveOccurred())
				validateSchedulerContext(schedCtx, 1, 1, sets.New[string](), sets.New[string]())
			})

			It("Multiple node maintenance with MaxUnavailable", func() {
				maintenances = []*maintenancev1.NodeMaintenance{
					// maintenances in progress
					testutils.GetTestNodeMaintenance("test-nm-0", "node-0", "test-operator.nvidia.com", maintenancev1.ConditionReasonScheduled),
					testutils.GetTestNodeMaintenance("test-nm-1", "node-1", "test-operator.nvidia.com", maintenancev1.ConditionReasonScheduled),
					// maintenances pending
					testutils.GetTestNodeMaintenance("test-nm-2", "node-0", "test-operator.nvidia.com", maintenancev1.ConditionReasonPending),
					testutils.GetTestNodeMaintenance("test-nm-3", "node-1", "test-operator.nvidia.com", maintenancev1.ConditionReasonPending),
					testutils.GetTestNodeMaintenance("test-nm-4", "node-2", "test-operator.nvidia.com", maintenancev1.ConditionReasonPending),
				}
				reconciler.Options.Store(&intstr.IntOrString{Type: intstr.Int, IntVal: 3},
					&intstr.IntOrString{Type: intstr.Int, IntVal: 5})
				reconciler.Options.Load()

				schedCtx, err := reconciler.preSchedule(nodes, maintenances)
				Expect(err).ToNot(HaveOccurred())
				validateSchedulerContext(schedCtx, 3, 1, sets.New("node-2"), sets.New("test-nm-4"))
			})

			It("Multiple node maintenance with MaxUnavailable Exceeds limit", func() {
				maintenances = []*maintenancev1.NodeMaintenance{
					// maintenances in progress
					testutils.GetTestNodeMaintenance("test-nm-0", "node-0", "test-operator.nvidia.com", maintenancev1.ConditionReasonScheduled),
					testutils.GetTestNodeMaintenance("test-nm-1", "node-1", "test-operator.nvidia.com", maintenancev1.ConditionReasonScheduled),
					// maintenances pending
					testutils.GetTestNodeMaintenance("test-nm-2", "node-2", "test-operator.nvidia.com", maintenancev1.ConditionReasonPending),
					testutils.GetTestNodeMaintenance("test-nm-3", "node-2", "test-operator.nvidia.com", maintenancev1.ConditionReasonPending),
				}
				reconciler.Options.Store(&intstr.IntOrString{Type: intstr.String, StrVal: "50%"},
					&intstr.IntOrString{Type: intstr.Int, IntVal: 5})
				reconciler.Options.Load()

				schedCtx, err := reconciler.preSchedule(nodes, maintenances)
				Expect(err).ToNot(HaveOccurred())
				validateSchedulerContext(schedCtx, 3, 0, sets.New("node-2"), sets.New("test-nm-2", "test-nm-3"))
			})

			It("Multiple node maintenance with MaxUnavailable Exceeds limit due to node unschedulable", func() {
				// node-0, node-1 are considered unavailable as they have maintenance scheduled
				maintenances = []*maintenancev1.NodeMaintenance{
					// maintenances in progress
					testutils.GetTestNodeMaintenance("test-nm-0", "node-0", "test-operator.nvidia.com", maintenancev1.ConditionReasonScheduled),
					testutils.GetTestNodeMaintenance("test-nm-1", "node-1", "test-operator.nvidia.com", maintenancev1.ConditionReasonDraining),
					// maintenances pending
					testutils.GetTestNodeMaintenance("test-nm-2", "node-1", "test-operator.nvidia.com", maintenancev1.ConditionReasonPending),
					testutils.GetTestNodeMaintenance("test-nm-3", "node-2", "test-operator.nvidia.com", maintenancev1.ConditionReasonPending),
				}
				// node-2 is unavailable because of "other" reason
				nodes[2].Spec.Unschedulable = true
				reconciler.Options.Store(&intstr.IntOrString{Type: intstr.Int, IntVal: 3},
					&intstr.IntOrString{Type: intstr.Int, IntVal: 5})
				reconciler.Options.Load()

				schedCtx, err := reconciler.preSchedule(nodes, maintenances)
				Expect(err).ToNot(HaveOccurred())
				validateSchedulerContext(schedCtx, 3, 0, sets.New("node-2"), sets.New("test-nm-3"))
			})

			It("Multiple node maintenance, MaxParallelOperations and MaxUnavailable Exceeds limit", func() {
				maintenances = []*maintenancev1.NodeMaintenance{
					// maintenances in progress
					testutils.GetTestNodeMaintenance("test-nm-0", "node-0", "test-operator.nvidia.com", maintenancev1.ConditionReasonScheduled),
					testutils.GetTestNodeMaintenance("test-nm-1", "node-1", "test-operator.nvidia.com", maintenancev1.ConditionReasonDraining),
					testutils.GetTestNodeMaintenance("test-nm-3", "node-2", "test-operator.nvidia.com", maintenancev1.ConditionReasonCordon),
					// maintenances pending
					testutils.GetTestNodeMaintenance("test-nm-4", "node-3", "test-operator.nvidia.com", maintenancev1.ConditionReasonPending),
					testutils.GetTestNodeMaintenance("test-nm-5", "node-4", "test-operator.nvidia.com", maintenancev1.ConditionReasonPending),
					testutils.GetTestNodeMaintenance("test-nm-6", "node-4", "test-operator.nvidia.com", maintenancev1.ConditionReasonPending),
				}
				reconciler.Options.Store(&intstr.IntOrString{Type: intstr.Int, IntVal: 2},
					&intstr.IntOrString{Type: intstr.Int, IntVal: 2})
				reconciler.Options.Load()

				schedCtx, err := reconciler.preSchedule(nodes, maintenances)
				Expect(err).ToNot(HaveOccurred())
				validateSchedulerContext(schedCtx, 0, 0, sets.New("node-3", "node-4"), sets.New("test-nm-4", "test-nm-5", "test-nm-6"))
			})

		})

		Context("NewNodeMaintenanceSchedulerReconcilerOptions", func() {
			It("Works", func() {
				options := NewNodeMaintenanceSchedulerReconcilerOptions()
				Expect(options.MaxParallelOperations()).To(Equal(defaultmaxParallelOperations))
				Expect(options.MaxUnavailable()).To(BeNil())
				newVal := &intstr.IntOrString{Type: intstr.Int, IntVal: 3}
				options.Store(newVal, newVal)
				Expect(options.MaxParallelOperations()).To(Equal(defaultmaxParallelOperations))
				Expect(options.MaxUnavailable()).To(BeNil())
				options.Load()
				Expect(options.MaxParallelOperations()).To(Equal(newVal))
				Expect(options.MaxUnavailable()).To(Equal(newVal))
			})
		})
	})
})

// validateSchedulerContext is a common function to validate SchedulerContext for tests
func validateSchedulerContext(
	schedCtx *scheduler.SchedulerContext,
	availSlots int,
	canBecomeUnavailable int,
	candidateNodes sets.Set[string],
	candidateMaintenance sets.Set[string]) {
	ExpectWithOffset(-1, schedCtx.AvailableSlots).To(Equal(availSlots))
	ExpectWithOffset(-1, schedCtx.CanBecomeUnavailable).To(Equal(canBecomeUnavailable))
	ExpectWithOffset(-1, schedCtx.CandidateNodes.Equal(candidateNodes)).
		To(BeTrue(), "candidate nodes does not match expected. diff:%v", schedCtx.CandidateNodes.SymmetricDifference(candidateNodes))

	actualMaintenances := sets.New[string]()
	for _, nm := range schedCtx.CandidateMaintenance {
		actualMaintenances.Insert(nm.Name)
	}
	ExpectWithOffset(-1, actualMaintenances.Equal(candidateMaintenance)).
		To(BeTrue(), "candidate maintenances does not match expected. diff:%v", candidateMaintenance.SymmetricDifference(actualMaintenances))
}
