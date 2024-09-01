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
	"github.com/Mellanox/maintenance-operator/internal/testutils"
)

var _ = Describe("NodeMaintenance Controller", func() {
	Context("Envtests", func() {
		var nmObjectsToCleanup []*maintenancev1.NodeMaintenance
		var reconciler *NodeMaintenanceGarbageCollector
		var options *GarbageCollectorOptions
		// test context, TODO(adrianc): use ginkgo spec context
		var testCtx context.Context

		BeforeEach(func() {
			testCtx = context.Background()
			garbageCollectionReconcileTime = 100 * time.Millisecond
			DeferCleanup(func() {
				garbageCollectionReconcileTime = defaultGarbageCollectionReconcileTime
			})

			// create controller manager
			By("create controller manager")
			mgr, err := ctrl.NewManager(cfg, ctrl.Options{
				Scheme:     k8sClient.Scheme(),
				Metrics:    metricsserver.Options{BindAddress: "0"},
				Controller: ctrlconfig.Controller{SkipNameValidation: kptr.To(true)},
			})
			Expect(err).ToNot(HaveOccurred())

			// create reconciler
			By("create NodeMaintenanceGarbageCollector")
			options = NewGarbageCollectorOptions()
			options.Store(1 * time.Second)
			reconciler = NewNodeMaintenanceGarbageCollector(
				k8sClient, options, ctrllog.Log.WithName("NodeMaintenanceGarbageCollector"))

			// setup reconciler with manager
			By("setup NodeMaintenanceGarbageCollector with controller manager")
			Expect(reconciler.SetupWithManager(mgr)).
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
			By("Cleanup NodeMaintenance resources")
			for _, nm := range nmObjectsToCleanup {
				err := k8sClient.Delete(testCtx, nm)
				if err != nil && k8serrors.IsNotFound(err) {
					err = nil
				}
				Expect(err).ToNot(HaveOccurred())
			}
			By("Wait for NodeMaintenance resources to be deleted")
			for _, nm := range nmObjectsToCleanup {
				Eventually(func() bool {
					err := k8sClient.Get(testCtx, types.NamespacedName{Namespace: nm.Namespace, Name: nm.Name}, nm)
					if err != nil && k8serrors.IsNotFound(err) {
						return true
					}
					return false

				}).WithTimeout(10 * time.Second).WithPolling(1 * time.Second).Should(BeTrue())
			}
			nmObjectsToCleanup = make([]*maintenancev1.NodeMaintenance, 0)
		})

		It("Should Delete NodeMaintenance with ready time annotation", func() {
			nm := testutils.GetTestNodeMaintenance("test-nm", "test-node-0", "some-operator.nvidia.com", "")
			metav1.SetMetaDataAnnotation(&nm.ObjectMeta, ReadyTimeAnnotation, time.Now().UTC().Format(time.RFC3339))
			Expect(k8sClient.Create(testCtx, nm)).ToNot(HaveOccurred())
			nmObjectsToCleanup = append(nmObjectsToCleanup, nm)

			By("Consistently NodeMaintenance exists")
			Consistently(k8sClient.Get(testCtx, client.ObjectKeyFromObject(nm), nm)).
				Within(500 * time.Millisecond).
				WithPolling(100 * time.Millisecond).
				Should(Succeed())

			By("Eventually NodeMaintenance is deleted")
			Eventually(func() bool {
				err := k8sClient.Get(testCtx, client.ObjectKeyFromObject(nm), nm)
				if err != nil && k8serrors.IsNotFound(err) {
					return true
				}
				return false
			}).
				WithTimeout(1 * time.Second).
				Should(BeTrue())
		})

		It("should not delete NodeMaintenance with ready time annotation if ignore garbage collection annotation set", func() {
			nm := testutils.GetTestNodeMaintenance("test-nm", "test-node-0", "some-operator.nvidia.com", "")
			metav1.SetMetaDataAnnotation(&nm.ObjectMeta, ReadyTimeAnnotation, time.Now().UTC().Format(time.RFC3339))
			metav1.SetMetaDataAnnotation(&nm.ObjectMeta, GarbageCollectIgnoreAnnotation, "true")
			Expect(k8sClient.Create(testCtx, nm)).ToNot(HaveOccurred())
			nmObjectsToCleanup = append(nmObjectsToCleanup, nm)

			By("Consistently NodeMaintenance exists")
			Consistently(k8sClient.Get(testCtx, client.ObjectKeyFromObject(nm), nm)).
				Within(2 * time.Second).
				WithPolling(500 * time.Millisecond).
				Should(Succeed())
		})

		It("should not delete NodeMaintenance without ready time annotation", func() {
			nm := testutils.GetTestNodeMaintenance("test-nm", "test-node-0", "some-operator.nvidia.com", "")
			Expect(k8sClient.Create(testCtx, nm)).ToNot(HaveOccurred())
			nmObjectsToCleanup = append(nmObjectsToCleanup, nm)

			By("Consistently NodeMaintenance exists")
			Consistently(k8sClient.Get(testCtx, client.ObjectKeyFromObject(nm), nm)).
				Within(2 * time.Second).
				WithPolling(500 * time.Millisecond).
				Should(Succeed())
		})
	})

	Context("UnitTests", func() {
		Context("GarbageCollectorOptions", func() {
			It("Works", func() {
				options := NewGarbageCollectorOptions()
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
