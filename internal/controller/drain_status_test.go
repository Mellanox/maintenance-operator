/*
 Copyright 2026, NVIDIA CORPORATION & AFFILIATES

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
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	maintenancev1 "github.com/Mellanox/maintenance-operator/api/v1alpha1"
	"github.com/Mellanox/maintenance-operator/internal/drain"
	drainmocks "github.com/Mellanox/maintenance-operator/internal/drain/mocks"
	"github.com/Mellanox/maintenance-operator/internal/k8sutils"
	"github.com/Mellanox/maintenance-operator/internal/testutils"
)

var _ = Describe("Drain error status", func() {
	var ctx context.Context
	var nm *maintenancev1.NodeMaintenance
	var r *NodeMaintenanceReconciler

	BeforeEach(func() {
		ctx = context.Background()
		nm = testutils.GetTestNodeMaintenance("drain-error-status", "drain-error-node", "test.nvidia.com", "")
		nm.Spec.DrainSpec = &maintenancev1.DrainSpec{Force: true, DeleteEmptyDir: false, TimeoutSecond: 10}
		Expect(k8sClient.Create(ctx, nm)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, nm)).To(Succeed()) })
		Expect(k8sutils.SetReadyConditionReason(ctx, k8sClient, nm, maintenancev1.ConditionReasonDraining)).To(Succeed())
		r = &NodeMaintenanceReconciler{Client: k8sClient}
	})

	It("reports local storage blockers and clears the error after the drain spec is corrected", func() {
		pod := testutils.GetTestPod("local-storage-blocker", nm.Spec.NodeName, nil)
		pod.Spec.Volumes = []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}}}
		Expect(k8sClient.Create(ctx, pod)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, pod, client.GracePeriodSeconds(0))).To(Succeed()) })

		req := drain.NewDrainRequest(ctx, ctrllog.Log, k8sInterface, nm, false)
		DeferCleanup(req.CancelDrain)
		err := r.updateDrainStatus(ctx, nm, req)
		Expect(err).To(MatchError(ContainSubstring("cannot delete Pods with local storage")))
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(nm), nm)).To(Succeed())
		condition := meta.FindStatusCondition(nm.Status.Conditions, maintenancev1.ConditionTypeReady)
		Expect(condition).NotTo(BeNil())
		Expect(condition.Status).To(Equal(metav1.ConditionFalse))
		Expect(condition.Reason).To(Equal(maintenancev1.ConditionReasonDraining))
		Expect(condition.Message).To(ContainSubstring("default/local-storage-blocker"))
		Expect(nm.Status.Drain).To(BeNil())

		nm.Spec.DrainSpec.DeleteEmptyDir = true
		Expect(k8sClient.Update(ctx, nm)).To(Succeed())
		req = drain.NewDrainRequest(ctx, ctrllog.Log, k8sInterface, nm, false)
		DeferCleanup(req.CancelDrain)
		Expect(r.updateDrainStatus(ctx, nm, req)).To(Succeed())
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(nm), nm)).To(Succeed())
		condition = meta.FindStatusCondition(nm.Status.Conditions, maintenancev1.ConditionTypeReady)
		Expect(condition.Message).To(BeEmpty())
		Expect(condition.Status).To(Equal(metav1.ConditionFalse))
		Expect(condition.ObservedGeneration).To(Equal(nm.Generation))
		Expect(nm.Status.Drain.WaitForEviction).To(ConsistOf("default/local-storage-blocker"))
		Expect(nm.Status.Drain.DrainProgress).To(BeZero())
	})

	It("preserves the collection error when persisting its message conflicts", func() {
		stale := nm.DeepCopy()
		Expect(k8sutils.SetReadyConditionReasonMsg(ctx, k8sClient, nm, maintenancev1.ConditionReasonDraining, "newer status")).To(Succeed())
		req := drainmocks.NewDrainRequest(GinkgoT())
		collectionErr := errors.New("pod list unavailable")
		req.EXPECT().Status().Return(drain.DrainStatus{}, collectionErr)
		err := r.updateDrainStatus(ctx, stale, req)
		Expect(errors.Is(err, collectionErr)).To(BeTrue())
		Expect(err.Error()).To(ContainSubstring("the object has been modified"))
	})

	DescribeTable("reports completed drain errors before retrying", func(state drain.DrainState) {
		req := drainmocks.NewDrainRequest(GinkgoT())
		mgr := drainmocks.NewManager(GinkgoT())
		r.DrainManager = mgr
		uid := drain.DrainRequestUIDFromNodeMaintenance(nm)
		mgr.EXPECT().GetRequest(uid).Return(req)
		req.EXPECT().UID().Return(uid)
		req.EXPECT().State().Return(state)
		req.EXPECT().Spec().Return(drain.DrainRequestSpec{NodeName: nm.Spec.NodeName, Spec: *nm.Spec.DrainSpec})
		req.EXPECT().LastError().Return(errors.New("eviction timed out"))
		mgr.EXPECT().RemoveRequest(uid).Run(func(string) {
			stored := &maintenancev1.NodeMaintenance{}
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(nm), stored)).To(Succeed())
			condition := meta.FindStatusCondition(stored.Status.Conditions, maintenancev1.ConditionTypeReady)
			Expect(condition.Reason).To(Equal(maintenancev1.ConditionReasonDraining))
			Expect(condition.Status).To(Equal(metav1.ConditionFalse))
			Expect(condition.Message).To(Equal("eviction timed out"))
		})
		result, err := r.handleDrainState(ctx, ctrllog.Log, nm, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(Equal(drainReqeueTime))
	}, Entry("failed", drain.DrainStateError), Entry("canceled", drain.DrainStateCanceled))

	It("does not enqueue a retry for a drain message-only update", func() {
		old := nm.DeepCopy()
		updated := nm.DeepCopy()
		meta.FindStatusCondition(updated.Status.Conditions, maintenancev1.ConditionTypeReady).Message = "eviction timed out"
		predicate := NewConditionChangedPredicate(ctrllog.Log)

		Expect(predicate.Update(event.UpdateEvent{ObjectOld: old, ObjectNew: updated})).To(BeFalse())
		Expect(meta.FindStatusCondition(old.Status.Conditions, maintenancev1.ConditionTypeReady).Message).To(BeEmpty())
		Expect(meta.FindStatusCondition(updated.Status.Conditions, maintenancev1.ConditionTypeReady).Message).To(Equal("eviction timed out"))

		old, updated = updated, old
		Expect(predicate.Update(event.UpdateEvent{ObjectOld: old, ObjectNew: updated})).To(BeFalse())

		meta.FindStatusCondition(updated.Status.Conditions, maintenancev1.ConditionTypeReady).ObservedGeneration++
		Expect(predicate.Update(event.UpdateEvent{ObjectOld: old, ObjectNew: updated})).To(BeTrue())

		updated = nm.DeepCopy()
		meta.FindStatusCondition(updated.Status.Conditions, maintenancev1.ConditionTypeReady).Reason = maintenancev1.ConditionReasonReady
		Expect(predicate.Update(event.UpdateEvent{ObjectOld: old, ObjectNew: updated})).To(BeTrue())
	})
})
