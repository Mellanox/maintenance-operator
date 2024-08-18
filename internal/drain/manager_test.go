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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/Mellanox/maintenance-operator/internal/drain"
	"github.com/Mellanox/maintenance-operator/internal/drain/mocks"
)

var _ = Describe("manager tests", func() {
	var reqMock *mocks.DrainRequest
	var manager drain.Manager

	BeforeEach(func() {
		reqMock = mocks.NewDrainRequest(GinkgoT())
		manager = drain.NewManager(ctrllog.Log, context.TODO(), nil)
	})

	Context("Interface Functions", func() {
		uid := "abcd"

		It("Adds request successfully", func() {
			reqMock.EXPECT().UID().Return(uid)
			reqMock.EXPECT().StartDrain().Return().Once()
			Expect(manager.AddRequest(reqMock)).To(Succeed())
		})

		It("Fails to Add same request", func() {
			reqMock.EXPECT().UID().Return(uid)
			reqMock.EXPECT().StartDrain().Return().Once()
			Expect(manager.AddRequest(reqMock)).To(Succeed())
			err := manager.AddRequest(reqMock)
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(drain.ErrDrainRequestExists))
		})

		It("Gets request", func() {
			reqMock.EXPECT().UID().Return(uid)
			reqMock.EXPECT().StartDrain().Return().Once()
			Expect(manager.AddRequest(reqMock)).To(Succeed())

			req := manager.GetRequest(uid)
			Expect(req).To(BeEquivalentTo(reqMock))
		})

		It("Get non exitent request returns nil", func() {
			req := manager.GetRequest(uid)
			Expect(req).To(BeNil())
		})

		It("Remove request", func() {
			reqMock.EXPECT().UID().Return(uid)
			reqMock.EXPECT().StartDrain().Return().Once()
			reqMock.EXPECT().CancelDrain().Return().Once()
			Expect(manager.AddRequest(reqMock)).To(Succeed())
			manager.RemoveRequest(uid)
			req := manager.GetRequest(uid)
			Expect(req).To(BeNil())
			manager.RemoveRequest(uid)
		})

		It("List requests", func() {
			reqMock.EXPECT().UID().Return(uid)
			reqMock.EXPECT().StartDrain().Return().Once()
			Expect(manager.AddRequest(reqMock)).To(Succeed())
			Expect(manager.ListRequests()).To(HaveLen(1))
			Expect(manager.ListRequests()[0]).To(BeEquivalentTo(reqMock))
		})
	})
})
