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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	maintenancev1 "github.com/Mellanox/maintenance-operator/api/v1alpha1"
	operatorlog "github.com/Mellanox/maintenance-operator/internal/log"
)

var _ = Describe("MaintenanceOperatorConfig Controller", func() {
	var reconciler *MaintenanceOperatorConfigReconciler

	// test context, TODO(adrianc): use ginkgo spec context
	var testCtx context.Context

	BeforeEach(func() {
		testCtx = context.Background()

		// create controller manager
		By("create controller manager")
		mgr, err := ctrl.NewManager(cfg, ctrl.Options{
			Scheme:  k8sClient.Scheme(),
			Metrics: metricsserver.Options{BindAddress: "0"},
		})
		Expect(err).ToNot(HaveOccurred())

		// create reconciler
		By("create MaintenanceOperatorConfigReconciler")
		reconciler = &MaintenanceOperatorConfigReconciler{
			Client:                    k8sClient,
			Scheme:                    k8sClient.Scheme(),
			SchedulerReconcierOptions: NewNodeMaintenanceSchedulerReconcilerOptions(),
			GarbageCollectorOptions:   NewGarbageCollectorOptions(),
		}

		// setup reconciler with manager
		By("setup MaintenanceOperatorConfigReconciler with controller manager")
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

	It("Should Reconcile MaintenanceOperatorConfig resource with defaults", func() {
		oc := &maintenancev1.MaintenanceOperatorConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: "default"},
			Spec:       maintenancev1.MaintenanceOperatorConfigSpec{},
		}
		Expect(k8sClient.Create(testCtx, oc)).ToNot(HaveOccurred())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(testCtx, oc)).ToNot(HaveOccurred())
		})

		Consistently(func(g Gomega) {
			reconciler.SchedulerReconcierOptions.Load()
			reconciler.GarbageCollectorOptions.Load()
			g.Expect(reconciler.SchedulerReconcierOptions.MaxParallelOperations()).To(Equal(&intstr.IntOrString{Type: intstr.Int, IntVal: 1}))
			g.Expect(reconciler.SchedulerReconcierOptions.MaxUnavailable()).To(BeNil())
			g.Expect(reconciler.GarbageCollectorOptions.MaxNodeMaintenanceTime()).To(Equal(defaultMaxNodeMaintenanceTime))
		}).ProbeEvery(100 * time.Millisecond).Within(time.Second).Should(Succeed())
	})

	It("Should Reconcile MaintenanceOperatorConfig resource with specified values", func() {
		By("create MaintenanceOperatorConfig with non default values")
		oc := &maintenancev1.MaintenanceOperatorConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: "default"},
			Spec: maintenancev1.MaintenanceOperatorConfigSpec{
				MaxParallelOperations:         &intstr.IntOrString{Type: intstr.Int, IntVal: 3},
				MaxUnavailable:                &intstr.IntOrString{Type: intstr.Int, IntVal: 3},
				MaxNodeMaintenanceTimeSeconds: 300,
				LogLevel:                      "debug",
			},
		}
		Expect(k8sClient.Create(testCtx, oc)).ToNot(HaveOccurred())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(testCtx, oc)).ToNot(HaveOccurred())
		})

		By("check MaintenanceOperatorConfig values were updated")
		Eventually(func(g Gomega) {
			reconciler.SchedulerReconcierOptions.Load()
			reconciler.GarbageCollectorOptions.Load()
			g.Expect(reconciler.SchedulerReconcierOptions.MaxParallelOperations()).To(Equal(oc.Spec.MaxParallelOperations))
			g.Expect(reconciler.SchedulerReconcierOptions.MaxUnavailable()).To(Equal(oc.Spec.MaxUnavailable))
			g.Expect(reconciler.GarbageCollectorOptions.MaxNodeMaintenanceTime()).
				To(Equal(time.Second * time.Duration(oc.Spec.MaxNodeMaintenanceTimeSeconds)))
			g.Expect(operatorlog.GetLogLevel()).To(BeEquivalentTo(oc.Spec.LogLevel))
		}).ProbeEvery(100 * time.Millisecond).Within(time.Second).Should(Succeed())

		By("update MaintenanceOperatorConfig")
		oc.Spec.MaxParallelOperations = &intstr.IntOrString{Type: intstr.Int, IntVal: 5}
		oc.Spec.MaxUnavailable = nil
		oc.Spec.LogLevel = "info"
		Expect(k8sClient.Update(testCtx, oc)).ToNot(HaveOccurred())

		By("check MaintenanceOperatorConfig values were updated")
		Eventually(func(g Gomega) {
			reconciler.SchedulerReconcierOptions.Load()
			reconciler.GarbageCollectorOptions.Load()
			g.Expect(reconciler.SchedulerReconcierOptions.MaxParallelOperations()).To(Equal(oc.Spec.MaxParallelOperations))
			g.Expect(reconciler.SchedulerReconcierOptions.MaxUnavailable()).To(Equal(oc.Spec.MaxUnavailable))
			g.Expect(reconciler.GarbageCollectorOptions.MaxNodeMaintenanceTime()).
				To(Equal(time.Second * time.Duration(oc.Spec.MaxNodeMaintenanceTimeSeconds)))
			g.Expect(operatorlog.GetLogLevel()).To(BeEquivalentTo(oc.Spec.LogLevel))
		}).ProbeEvery(100 * time.Millisecond).Within(time.Second).Should(Succeed())
	})
})
