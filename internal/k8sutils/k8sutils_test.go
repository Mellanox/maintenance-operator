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

package k8sutils_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	maintenancev1 "github.com/Mellanox/maintenance-operator/api/v1alpha1"
	"github.com/Mellanox/maintenance-operator/internal/k8sutils"
	"github.com/Mellanox/maintenance-operator/internal/testutils"
)

var _ = Describe("k8sutils Tests", func() {
	var fakeClient client.Client
	var testCtx context.Context

	BeforeEach(func() {
		testCtx = context.Background()
		s := runtime.NewScheme()
		maintenancev1.AddToScheme(s)
		corev1.AddToScheme(s)
		fakeClient = fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(&maintenancev1.NodeMaintenance{}).Build()
	})

	Context("GetReadyConditionReason", func() {
		It("Gets condition as expected", func() {
			nm := testutils.GetTestNodeMaintenance("test", "node-1", "test.nvidia.com", "")
			Expect(k8sutils.GetReadyConditionReason(nm)).To(Equal(maintenancev1.ConditionReasonUninitialized))
			Expect(fakeClient.Create(testCtx, nm)).ToNot(HaveOccurred())
			Expect(k8sutils.SetReadyConditionReason(testCtx, fakeClient, nm, maintenancev1.ConditionReasonPending)).ToNot(HaveOccurred())
			Expect(k8sutils.GetReadyConditionReason(nm)).To(Equal(maintenancev1.ConditionReasonPending))
		})
	})

	Context("SetReadyConditionReason", func() {
		It("Sets condition as expected", func() {
			nm := testutils.GetTestNodeMaintenance("test", "node-1", "test.nvidia.com", "")
			nm.Generation = 4
			Expect(fakeClient.Create(testCtx, nm)).ToNot(HaveOccurred())
			Expect(k8sutils.SetReadyConditionReason(testCtx, fakeClient, nm, maintenancev1.ConditionReasonCordon)).ToNot(HaveOccurred())
			By("Object is updated in place")
			Expect(nm.Status.Conditions[0].Type).To(Equal(maintenancev1.ConditionTypeReady))
			Expect(nm.Status.Conditions[0].Reason).To(Equal(maintenancev1.ConditionReasonCordon))
			Expect(nm.Status.Conditions[0].ObservedGeneration).To(Equal(nm.Generation))
			By("Object is updated in k8s")
			Expect(fakeClient.Get(testCtx, types.NamespacedName{Namespace: nm.Namespace, Name: nm.Name}, nm)).ToNot(HaveOccurred())
			Expect(nm.Status.Conditions[0].Type).To(Equal(maintenancev1.ConditionTypeReady))
			Expect(nm.Status.Conditions[0].Reason).To(Equal(maintenancev1.ConditionReasonCordon))
			Expect(nm.Status.Conditions[0].ObservedGeneration).To(Equal(nm.Generation))
		})
	})

	Context("SetReadyConditionReasonMsg", func() {
		It("Sets condition as expected with msg", func() {
			nm := testutils.GetTestNodeMaintenance("test", "node-1", "test.nvidia.com", "")
			Expect(fakeClient.Create(testCtx, nm)).ToNot(HaveOccurred())
			By("updating status should succeed")
			Expect(k8sutils.SetReadyConditionReasonMsg(testCtx, fakeClient, nm, maintenancev1.ConditionReasonCordon, "foobar")).ToNot(HaveOccurred())
			Expect(nm.Status.Conditions[0].Type).To(Equal(maintenancev1.ConditionTypeReady))
			Expect(nm.Status.Conditions[0].Reason).To(Equal(maintenancev1.ConditionReasonCordon))
			Expect(nm.Status.Conditions[0].Message).To(Equal("foobar"))
			By("updating again to same value should succeed without calling k8s client")
			Expect(k8sutils.SetReadyConditionReasonMsg(testCtx, nil, nm, maintenancev1.ConditionReasonCordon, "foobar")).ToNot(HaveOccurred())
			Expect(nm.Status.Conditions[0].Type).To(Equal(maintenancev1.ConditionTypeReady))
			Expect(nm.Status.Conditions[0].Reason).To(Equal(maintenancev1.ConditionReasonCordon))
			Expect(nm.Status.Conditions[0].Message).To(Equal("foobar"))
		})
	})

	Context("IsUnderMaintenance", func() {
		var nm *maintenancev1.NodeMaintenance

		BeforeEach(func() {
			nm = &maintenancev1.NodeMaintenance{}
		})

		It("Returns false if not under maintenance", func() {
			By("no condition")
			nm.Status.Conditions = nil
			Expect(k8sutils.IsUnderMaintenance(nm)).To(BeFalse())
			By("ready condition with pending reason")
			nm.Status.Conditions = []metav1.Condition{
				{
					Type:   maintenancev1.ConditionTypeReady,
					Status: metav1.ConditionFalse,
					Reason: maintenancev1.ConditionReasonPending,
				},
			}
			Expect(k8sutils.IsUnderMaintenance(nm)).To(BeFalse())
		})

		It("Returns true if under maintenance", func() {
			nm.Status.Conditions = []metav1.Condition{
				{
					Type:   maintenancev1.ConditionTypeReady,
					Status: metav1.ConditionFalse,
					Reason: maintenancev1.ConditionReasonScheduled,
				},
			}
			Expect(k8sutils.IsUnderMaintenance(nm)).To(BeTrue())
		})
	})

	Context("IsNodeReady", func() {
		var node *corev1.Node

		BeforeEach(func() {
			node = &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}}
		})

		It("Returns true when node is Ready", func() {
			node.Status.Conditions = []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}}
			Expect(k8sutils.IsNodeReady(node)).To(BeTrue())
		})

		It("Returns false when node is not Ready", func() {
			node.Status.Conditions = []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionFalse}}
			Expect(k8sutils.IsNodeReady(node)).To(BeFalse())
		})
	})

	Context("IsNodeUnschedulable", func() {
		var node *corev1.Node

		BeforeEach(func() {
			node = &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}}
		})

		It("Returns true when node is unschedulable", func() {
			node.Spec.Unschedulable = true
			Expect(k8sutils.IsNodeUnschedulable(node)).To(BeTrue())
		})

		It("Returns false when node is unschedulable", func() {
			node.Spec.Unschedulable = false
			Expect(k8sutils.IsNodeUnschedulable(node)).To(BeFalse())
		})
	})

	Context("AddFinalizer", func() {
		var nm *maintenancev1.NodeMaintenance

		BeforeEach(func() {
			nm = testutils.GetTestNodeMaintenance("test", "node-1", "test.nvidia.com", "")
		})

		It("adds finalizer to object and updates in k8s", func() {
			Expect(fakeClient.Create(testCtx, nm)).ToNot(HaveOccurred())
			Expect(k8sutils.AddFinalizer(testCtx, fakeClient, nm, "test-finalizer")).ToNot(HaveOccurred())
			Expect(nm.Finalizers).To(ContainElement("test-finalizer"))
			Expect(fakeClient.Get(testCtx, types.NamespacedName{Namespace: nm.Namespace, Name: nm.Name}, nm)).ToNot(HaveOccurred())
			Expect(nm.Finalizers).To(ContainElement("test-finalizer"))
			Expect(nm.Finalizers).To(HaveLen(1))
		})

		It("does nothing if finalizer exists", func() {
			nm.Finalizers = append(nm.Finalizers, "test-finalizer")
			Expect(k8sutils.AddFinalizer(testCtx, nil, nm, "test-finalizer")).ToNot(HaveOccurred())
			Expect(nm.Finalizers).To(ContainElement("test-finalizer"))
			Expect(nm.Finalizers).To(HaveLen(1))
		})
	})

	Context("RemoveFinalizer", func() {
		var nm *maintenancev1.NodeMaintenance

		BeforeEach(func() {
			nm = testutils.GetTestNodeMaintenance("test", "node-1", "test.nvidia.com", "")
		})

		It("does nothing if finalizer does not exits", func() {
			nm.Finalizers = append(nm.Finalizers, "foo")
			Expect(k8sutils.RemoveFinalizer(testCtx, nil, nm, "test-finalizer")).ToNot(HaveOccurred())
			Expect(nm.Finalizers).To(ContainElement("foo"))
			Expect(nm.Finalizers).To(HaveLen(1))
		})

		It("removes finalizer if exists", func() {
			nm.Finalizers = append(nm.Finalizers, "test-finalizer")
			Expect(fakeClient.Create(testCtx, nm)).ToNot(HaveOccurred())
			Expect(k8sutils.RemoveFinalizer(testCtx, fakeClient, nm, "test-finalizer")).ToNot(HaveOccurred())
			Expect(nm.Finalizers).To(BeEmpty())
			Expect(fakeClient.Get(testCtx, types.NamespacedName{Namespace: nm.Namespace, Name: nm.Name}, nm)).
				ToNot(HaveOccurred())
			Expect(nm.Finalizers).To(BeEmpty())
		})
	})

	Context("SetOwnerRef", func() {
		var node *corev1.Node
		var nm *maintenancev1.NodeMaintenance

		BeforeEach(func() {
			node = &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1", UID: "abcdef"}}
			nm = testutils.GetTestNodeMaintenance("test", "node-1", "foo.bar", "")
			nm.UID = "efgh"
		})

		It("sets owner reference for object", func() {
			Expect(fakeClient.Create(testCtx, nm)).ToNot(HaveOccurred())
			Expect(k8sutils.SetOwnerRef(testCtx, fakeClient, node, nm)).ToNot(HaveOccurred())
			Expect(nm.OwnerReferences[0].UID).To(Equal(node.UID))
		})
	})

	Context("HasOwnerRef", func() {
		var node *corev1.Node
		var nm *maintenancev1.NodeMaintenance

		BeforeEach(func() {
			node = &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1", UID: "abcdef"}}
			nm = testutils.GetTestNodeMaintenance("test", "node-1", "foo.bar", "")
			nm.UID = "efghij"
		})

		It("returns true if object has owner reference to owned", func() {
			nm.OwnerReferences = []metav1.OwnerReference{{UID: node.UID}}
			Expect(k8sutils.HasOwnerRef(node, nm)).To(BeTrue())
		})

		It("returns false if object does not have owner reference to owned", func() {
			Expect(k8sutils.HasOwnerRef(node, nm)).To(BeFalse())
		})
	})
})
