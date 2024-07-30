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
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	maintenancev1 "github.com/Mellanox/maintenance-operator/api/v1alpha1"
	"github.com/Mellanox/maintenance-operator/internal/k8sutils"
	"github.com/Mellanox/maintenance-operator/internal/testutils"
)

var _ = Describe("NodeMaintenance Controller", func() {
	Context("Envtests", func() {
		var nmObjectsToCleanup []*maintenancev1.NodeMaintenance
		var nodeObjectsToCleanup []*corev1.Node

		var reconciler *NodeMaintenanceReconciler
		var options *NodeMaintenanceReconcilerOptions

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
				Scheme:  k8sClient.Scheme(),
				Metrics: metricsserver.Options{BindAddress: "0"},
			})
			Expect(err).ToNot(HaveOccurred())

			// create reconciler
			By("create NodeMaintenanceReconciler")
			options = NewNodeMaintenanceReconcilerOptions()
			reconciler = &NodeMaintenanceReconciler{
				Client:  k8sClient,
				Scheme:  k8sClient.Scheme(),
				Options: options,
			}

			// setup reconciler with manager
			By("setup NodeMaintenanceReconciler with controller manager")
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

		It("Should transition new NodeMaintenance Resource to Pending", func() {
			nm := testutils.GetTestNodeMaintenance("test-nm", "test-node-0", "some-operator.nvidia.com", "")
			Expect(k8sClient.Create(testCtx, nm)).ToNot(HaveOccurred())
			nmObjectsToCleanup = append(nmObjectsToCleanup, nm)

			By("Eventually NodeMaintenance condition is set to Pending")
			Eventually(func() string {
				nm := &maintenancev1.NodeMaintenance{}
				err := k8sClient.Get(testCtx, types.NamespacedName{Namespace: "default", Name: "test-nm"}, nm)
				if err == nil {
					return k8sutils.GetReadyConditionReason(nm)
				}
				return ""
			}).WithTimeout(10 * time.Second).WithPolling(1 * time.Second).Should(Equal(maintenancev1.ConditionReasonPending))

			By("ConditionChanged event with Pending msg is sent for NodeMaintenance")
			Eventually(func() string {
				el := &corev1.EventList{}
				err := k8sClient.List(testCtx, el, client.MatchingFields{"involvedObject.uid": string(nm.UID)})
				if err == nil && len(el.Items) > 0 {
					return el.Items[0].Message
				}
				return ""
			}).WithTimeout(10 * time.Second).WithPolling(1 * time.Second).Should(Equal(maintenancev1.ConditionReasonPending))
		})
	})

	Context("UnitTests", func() {
		Context("NodeMaintenanceReconcilerOptions", func() {
			It("Works", func() {
				options := NewNodeMaintenanceReconcilerOptions()
				Expect(options.MaxNodeMaintenanceTime()).To(Equal(defaultMaxNodeMaintenanceTime))
				newTime := 300 * time.Second
				options.Store(newTime)
				Expect(options.MaxNodeMaintenanceTime()).To(Equal(defaultMaxNodeMaintenanceTime))
				options.Load()
				Expect(options.MaxNodeMaintenanceTime()).To(Equal(newTime))
			})
		})
	})

})
