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

package podcompletion_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	maintenancev1 "github.com/Mellanox/maintenance-operator/api/v1alpha1"
	operatorlog "github.com/Mellanox/maintenance-operator/internal/log"
	"github.com/Mellanox/maintenance-operator/internal/podcompletion"
	"github.com/Mellanox/maintenance-operator/internal/testutils"
)

var _ = BeforeSuite(func() {
	operatorlog.InitLog()
})

var _ = Describe("podcompletion tests", func() {
	var fakeClient client.Client
	var testCtx context.Context

	var nm *maintenancev1.NodeMaintenance
	var handler podcompletion.Handler

	BeforeEach(func() {
		testCtx = context.Background()
		nm = testutils.GetTestNodeMaintenance("test-nm", "node-0", "test.nvidia.com", "")
		nm.Spec.WaitForPodCompletion = &maintenancev1.WaitForPodCompletionSpec{}

		testPod := testutils.GetTestPod("test-pod", "node-0", map[string]string{"foo": "bar"})

		s := runtime.NewScheme()
		corev1.AddToScheme(s)
		maintenancev1.AddToScheme(s)
		fakeClient = ctrfake.NewClientBuilder().
			WithScheme(s).
			WithStatusSubresource(&maintenancev1.NodeMaintenance{}).
			WithObjects(nm, testPod).
			WithIndex(&corev1.Pod{}, "spec.nodeName", func(o client.Object) []string {
				return []string{o.(*corev1.Pod).Spec.NodeName}
			}).
			Build()
		handler = podcompletion.NewPodCompletionHandler(fakeClient)
	})

	Context("Test HandlePodCompletion", func() {
		It("returns expected list of pods and updates start time annotation", func() {
			nm.Spec.WaitForPodCompletion.PodSelector = ""
			Expect(fakeClient.Update(testCtx, nm)).ToNot(HaveOccurred())
			pods, err := handler.HandlePodCompletion(testCtx, ctrllog.Log.WithName("podCompletionHandler"), nm)
			Expect(err).ToNot(HaveOccurred())
			Expect(pods).To(Equal([]string{"default/test-pod"}))

			Expect(fakeClient.Get(testCtx, client.ObjectKeyFromObject(nm), nm)).ToNot(HaveOccurred())
			Expect(nm.Status.WaitForCompletion).To(Equal([]string{"default/test-pod"}))
			t1, e := time.Parse(time.RFC3339, nm.Annotations[podcompletion.WaitForPodCompletionStartAnnot])
			Expect(e).ToNot(HaveOccurred())

			// call again, expect wait-pod-completion-start annotation value to remain the same
			time.Sleep(1 * time.Second)
			_, err = handler.HandlePodCompletion(testCtx, ctrllog.Log.WithName("podCompletionHandler"), nm)
			Expect(err).ToNot(HaveOccurred())
			Expect(fakeClient.Get(testCtx, client.ObjectKeyFromObject(nm), nm)).ToNot(HaveOccurred())
			t2, e := time.Parse(time.RFC3339, nm.Annotations[podcompletion.WaitForPodCompletionStartAnnot])
			Expect(e).ToNot(HaveOccurred())
			Expect(t1.Equal(t2)).To(BeTrue())
		})

		It("returns pod completion timeout error if timeout happened if there are still pods", func() {
			nm.Spec.WaitForPodCompletion.TimeoutSecond = 1
			metav1.SetMetaDataAnnotation(&nm.ObjectMeta, podcompletion.WaitForPodCompletionStartAnnot,
				time.Now().Add(-2*time.Second).Format(time.RFC3339))
			_, err := handler.HandlePodCompletion(testCtx, ctrllog.Log.WithName("podCompletionHandler"), nm)
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(podcompletion.ErrPodCompletionTimeout))
		})

		It("succeeds if there are no pods to be waited", func() {
			nm.Spec.WaitForPodCompletion.PodSelector = "bar=baz"
			Expect(fakeClient.Update(testCtx, nm)).ToNot(HaveOccurred())
			pods, err := handler.HandlePodCompletion(testCtx, ctrllog.Log.WithName("podCompletionHandler"), nm)
			Expect(err).ToNot(HaveOccurred())
			Expect(pods).To(BeEmpty())
		})
	})
})
