/*
 Copyright 2025, NVIDIA CORPORATION & AFFILIATES

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

package openshift_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	ocpconfigv1 "github.com/openshift/api/config/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/Mellanox/maintenance-operator/internal/openshift"
)

var _ = Describe("openshift utils tests", func() {
	var testCtx context.Context
	var cFunc context.CancelFunc
	var s *runtime.Scheme

	BeforeEach(func() {
		testCtx, cFunc = context.WithCancel(context.TODO())
		s = runtime.NewScheme()
	})

	AfterEach(func() {
		cFunc()
	})

	Context("OpenshiftUtils when cluster is openshift", func() {
		It("should return correct values", func() {
			ocpconfigv1.Install(s)
			fakeClient := ctrfake.NewClientBuilder().
				WithScheme(s).
				WithStatusSubresource(&ocpconfigv1.Infrastructure{}).
				WithObjects(getInfrastructureResource(false)).
				Build()
			ocpUtils, err := openshift.NewOpenshiftUtils(testCtx, fakeClient)
			Expect(err).ToNot(HaveOccurred())
			Expect(ocpUtils.IsOpenshift()).To(BeTrue())
			Expect(ocpUtils.IsHypershift()).To(BeFalse())
		})
	})

	Context("OpenshiftUtils when cluster is hypershift", func() {
		It("should return correct values", func() {
			ocpconfigv1.Install(s)
			fakeClient := ctrfake.NewClientBuilder().
				WithScheme(s).
				WithStatusSubresource(&ocpconfigv1.Infrastructure{}).
				WithObjects(getInfrastructureResource(true)).
				Build()
			ocpUtils, err := openshift.NewOpenshiftUtils(testCtx, fakeClient)
			Expect(err).ToNot(HaveOccurred())
			Expect(ocpUtils.IsOpenshift()).To(BeTrue())
			Expect(ocpUtils.IsHypershift()).To(BeTrue())
		})
	})
})

func getInfrastructureResource(isHypershift bool) *ocpconfigv1.Infrastructure {
	cplaneTopology := ocpconfigv1.SingleReplicaTopologyMode
	if isHypershift {
		cplaneTopology = ocpconfigv1.ExternalTopologyMode
	}

	return &ocpconfigv1.Infrastructure{
		ObjectMeta: metav1.ObjectMeta{
			Name: openshift.InfraResourceName,
		},
		Spec: ocpconfigv1.InfrastructureSpec{},
		Status: ocpconfigv1.InfrastructureStatus{
			ControlPlaneTopology: cplaneTopology,
		},
	}
}
