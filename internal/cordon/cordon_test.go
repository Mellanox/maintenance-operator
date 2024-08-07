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

package cordon_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ks8fake "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	maintenancev1 "github.com/Mellanox/maintenance-operator/api/v1alpha1"
	"github.com/Mellanox/maintenance-operator/internal/cordon"
	operatorlog "github.com/Mellanox/maintenance-operator/internal/log"
	"github.com/Mellanox/maintenance-operator/internal/testutils"
)

var _ = BeforeSuite(func() {
	operatorlog.InitLog()
})

var _ = Describe("cordon tests", func() {
	var fakeClient client.Client
	var fakeInterface kubernetes.Interface
	var testCtx context.Context

	var node *corev1.Node
	var nm *maintenancev1.NodeMaintenance
	var handler cordon.Handler

	BeforeEach(func() {
		testCtx = context.Background()
		node = testutils.GetTestNodes("node", 1, false)[0]
		nm = testutils.GetTestNodeMaintenance("test-nm", node.Name, "test.nvidia.com", "")

		s := runtime.NewScheme()
		maintenancev1.AddToScheme(s)
		fakeClient = ctrfake.NewClientBuilder().
			WithScheme(s).
			WithStatusSubresource(&maintenancev1.NodeMaintenance{}).
			WithObjects(nm).
			Build()
		fakeInterface = ks8fake.NewSimpleClientset(node)
		handler = cordon.NewCordonHandler(fakeClient, fakeInterface)
	})

	Context("Test HandleCordon", func() {
		It("cordons node", func() {
			Expect(handler.HandleCordon(testCtx, ctrllog.Log.WithName("cordonHandler"), nm, node)).ToNot(HaveOccurred())

			Expect(fakeClient.Get(testCtx, types.NamespacedName{Namespace: nm.Namespace, Name: nm.Name}, nm)).ToNot(HaveOccurred())
			Expect(metav1.HasAnnotation(nm.ObjectMeta, cordon.NodeInitialStateUnschedulableAnnot)).To(BeTrue())
			Expect(nm.Annotations[cordon.NodeInitialStateUnschedulableAnnot]).To(Equal("false"))
			node, err := fakeInterface.CoreV1().Nodes().Get(testCtx, node.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(node.Spec.Unschedulable).To(BeTrue())
		})

		It("does not cordon node if initial state annotation is set to true", func() {
			metav1.SetMetaDataAnnotation(&nm.ObjectMeta, cordon.NodeInitialStateUnschedulableAnnot, "true")
			Expect(handler.HandleCordon(testCtx, ctrllog.Log.WithName("cordonHandler"), nm, node)).ToNot(HaveOccurred())

			node, err := fakeInterface.CoreV1().Nodes().Get(testCtx, node.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(node.Spec.Unschedulable).To(BeFalse())
		})

		It("succeeds multiple calls to HandleCordon", func() {
			Expect(handler.HandleCordon(testCtx, ctrllog.Log.WithName("cordonHandler"), nm, node)).ToNot(HaveOccurred())
			Expect(handler.HandleCordon(testCtx, ctrllog.Log.WithName("cordonHandler"), nm, node)).ToNot(HaveOccurred())

			node, err := fakeInterface.CoreV1().Nodes().Get(testCtx, node.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(node.Spec.Unschedulable).To(BeTrue())
		})
	})

	Context("Test HandleUnCordon", func() {
		It("uncordons node and removes annotation", func() {
			Expect(handler.HandleCordon(testCtx, ctrllog.Log.WithName("cordonHandler"), nm, node)).ToNot(HaveOccurred())
			Expect(handler.HandleUnCordon(testCtx, ctrllog.Log.WithName("cordonHandler"), nm, node)).ToNot(HaveOccurred())

			node, err := fakeInterface.CoreV1().Nodes().Get(testCtx, node.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(node.Spec.Unschedulable).To(BeFalse())
			Expect(fakeClient.Get(testCtx, types.NamespacedName{Namespace: nm.Namespace, Name: nm.Name}, nm)).ToNot(HaveOccurred())
			Expect(metav1.HasAnnotation(nm.ObjectMeta, cordon.NodeInitialStateUnschedulableAnnot)).To(BeFalse())

		})

		It("succeeds if node is not cordoned", func() {
			Expect(handler.HandleUnCordon(testCtx, ctrllog.Log.WithName("cordonHandler"), nm, node)).ToNot(HaveOccurred())

			node, err := fakeInterface.CoreV1().Nodes().Get(testCtx, node.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(node.Spec.Unschedulable).To(BeFalse())
			Expect(fakeClient.Get(testCtx, types.NamespacedName{Namespace: nm.Namespace, Name: nm.Name}, nm)).ToNot(HaveOccurred())
			Expect(metav1.HasAnnotation(nm.ObjectMeta, cordon.NodeInitialStateUnschedulableAnnot)).To(BeFalse())
		})

		It("uncordons multiple calls", func() {
			Expect(handler.HandleCordon(testCtx, ctrllog.Log.WithName("cordonHandler"), nm, node)).ToNot(HaveOccurred())
			Expect(handler.HandleUnCordon(testCtx, ctrllog.Log.WithName("cordonHandler"), nm, node)).ToNot(HaveOccurred())
			Expect(handler.HandleUnCordon(testCtx, ctrllog.Log.WithName("cordonHandler"), nm, node)).ToNot(HaveOccurred())
		})

		It("succeeds if node is not cordoned with initial state annotation", func() {
			metav1.SetMetaDataAnnotation(&nm.ObjectMeta, cordon.NodeInitialStateUnschedulableAnnot, "false")
			Expect(fakeClient.Update(testCtx, nm)).ToNot(HaveOccurred())
			Expect(handler.HandleUnCordon(testCtx, ctrllog.Log.WithName("cordonHandler"), nm, node)).ToNot(HaveOccurred())

			node, err := fakeInterface.CoreV1().Nodes().Get(testCtx, node.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(node.Spec.Unschedulable).To(BeFalse())
			Expect(fakeClient.Get(testCtx, types.NamespacedName{Namespace: nm.Namespace, Name: nm.Name}, nm)).ToNot(HaveOccurred())
			Expect(metav1.HasAnnotation(nm.ObjectMeta, cordon.NodeInitialStateUnschedulableAnnot)).To(BeFalse())
		})

		It("does not uncordon node if initial state annotation is true", func() {
			metav1.SetMetaDataAnnotation(&nm.ObjectMeta, cordon.NodeInitialStateUnschedulableAnnot, "true")
			node.Spec.Unschedulable = true
			_, err := fakeInterface.CoreV1().Nodes().Update(testCtx, node, metav1.UpdateOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(handler.HandleUnCordon(testCtx, ctrllog.Log.WithName("cordonHandler"), nm, node)).ToNot(HaveOccurred())
			node, err = fakeInterface.CoreV1().Nodes().Get(testCtx, node.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(node.Spec.Unschedulable).To(BeTrue())
		})
	})

})
