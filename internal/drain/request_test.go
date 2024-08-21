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

package drain_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	ks8fake "k8s.io/client-go/kubernetes/fake"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	maintenancev1 "github.com/Mellanox/maintenance-operator/api/v1alpha1"
	"github.com/Mellanox/maintenance-operator/internal/drain"
	"github.com/Mellanox/maintenance-operator/internal/testutils"
)

var _ = Describe("DrainRequest tests", func() {
	var fakeInterface kubernetes.Interface
	var testCtx context.Context
	var cFunc context.CancelFunc

	var node *corev1.Node
	var pod *corev1.Pod
	var nm *maintenancev1.NodeMaintenance
	var drainReq drain.DrainRequest

	BeforeEach(func() {
		testCtx, cFunc = context.WithCancel(context.TODO())
		node = testutils.GetTestNodes("node", 1, false)[0]
		pod = testutils.GetTestPod("test-pod-1", "node-0", map[string]string{"foo": "bar"})
		podWithResource := testutils.GetTestPod("test-pod-2", "node-0", nil)
		podWithResource.Spec.Containers[0].Resources.Requests = map[corev1.ResourceName]resource.Quantity{"nvidia.com/rdma-vf": {}}

		nm = testutils.GetTestNodeMaintenance("test-nm", node.Name, "test.nvidia.com", "")
		nm.Spec.DrainSpec = &maintenancev1.DrainSpec{
			Force:         true,
			PodSelector:   "",
			TimeoutSecond: 10,
		}

		fakeInterface = ks8fake.NewSimpleClientset(node, pod, podWithResource)
		// since we use fake client, we need to disable eviction for drain op to succeed.
		// in addition, fake client doesnt take into account context so no tests that rely
		// on ctx being cancelled cannot be tested.
		drainReq = drain.NewDrainRequest(testCtx, ctrllog.Log.WithName("test-drain"), fakeInterface, nm, true)
	})

	AfterEach(func() {
		cFunc()
	})

	Context("UID()", func() {
		It("returns expected UID", func() {
			Expect(drainReq.UID()).To(Equal(drain.DrainRequestUIDFromNodeMaintenance(nm)))
		})
	})

	Context("Spec()", func() {
		It("returns expected Spec", func() {
			spec := drainReq.Spec()
			Expect(spec.NodeName).To(Equal(node.Name))
			Expect(spec.Spec.TimeoutSecond).To(Equal(int32(10)))
		})
	})

	Context("Before Drain Start", func() {
		It("returns State Not Started", func() {
			Expect(drainReq.State()).To(Equal(drain.DrainStateNotStarted))
		})

		It("returns initial Status", func() {
			ds, err := drainReq.Status()
			Expect(err).ToNot(HaveOccurred())
			Expect(ds.State).To(Equal(drain.DrainStateNotStarted))
			Expect(ds.PodsToDelete).To(HaveLen(2))
			Expect(ds.PodsToDelete).To(ConsistOf("default/test-pod-1", "default/test-pod-2"))
		})
	})

	Context("StartDrain", func() {
		It("Drains Successfully", func() {
			drainReq.StartDrain()
			Eventually(drainReq.State).WithTimeout(1 * time.Second).WithPolling(100 * time.Millisecond).Should(Equal(drain.DrainStateSuccess))
			ds, err := drainReq.Status()
			Expect(err).ToNot(HaveOccurred())
			Expect(ds.State).To(Equal(drain.DrainStateSuccess))
			Expect(ds.PodsToDelete).To(HaveLen(0))
		})
	})

	Context("PodEvictionFiters", func() {
		BeforeEach(func() {
			regex := "nvidia.com/rdma-*"
			nm.Spec.DrainSpec.PodEvictionFilters = []maintenancev1.PodEvictionFiterEntry{
				{ByResourceNameRegex: &regex},
			}
			drainReq = drain.NewDrainRequest(testCtx, ctrllog.Log.WithName("test-drain"), fakeInterface, nm, true)
		})

		It("Returns expected status", func() {
			ds, err := drainReq.Status()
			Expect(err).ToNot(HaveOccurred())
			Expect(ds.PodsToDelete).To(HaveLen(1))
			Expect(ds.PodsToDelete).To(ConsistOf("default/test-pod-2"))
		})

		It("Drains only pods targeted by filters", func() {
			drainReq.StartDrain()
			Eventually(drainReq.State).WithTimeout(1 * time.Second).WithPolling(100 * time.Millisecond).Should(Equal(drain.DrainStateSuccess))
			ds, err := drainReq.Status()
			Expect(err).ToNot(HaveOccurred())
			Expect(ds.State).To(Equal(drain.DrainStateSuccess))
			Expect(ds.PodsToDelete).To(HaveLen(0))

			_, err = fakeInterface.CoreV1().Pods("default").Get(testCtx, "test-pod-1", metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("CancelDrain", func() {
		It("Sets DrainRequest state to canceled", func() {
			drainReq.CancelDrain()
			Expect(drainReq.State()).To(Equal(drain.DrainStateCanceled))
			// run again to ensure we can run cancel multiple times
			drainReq.CancelDrain()
			Expect(drainReq.State()).To(Equal(drain.DrainStateCanceled))
			// start drain should have no effect on drain state after cancel
			drainReq.StartDrain()
			Expect(drainReq.State()).To(Equal(drain.DrainStateCanceled))
		})
	})
})
