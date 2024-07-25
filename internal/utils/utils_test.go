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

package utils_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	maintenancev1 "github.com/Mellanox/maintenance-operator/api/v1alpha1"
	"github.com/Mellanox/maintenance-operator/internal/utils"
)

var _ = Describe("Utils Tests", func() {
	Context("MinInt", func() {
		It("returns minimum of two integers", func() {
			Expect(utils.MinInt(5, 6)).To(Equal(5))
			Expect(utils.MinInt(6, 5)).To(Equal(5))
		})
	})

	Context("MaxInt", func() {
		It("returns maximum of two integers", func() {
			Expect(utils.MaxInt(5, 6)).To(Equal(6))
			Expect(utils.MaxInt(6, 5)).To(Equal(6))
		})
	})

	Context("CanonicalStringsFromList*", func() {
		elm1 := maintenancev1.NodeMaintenance{
			ObjectMeta: v1.ObjectMeta{Name: "foo", Namespace: "ns1"},
			Spec: maintenancev1.NodeMaintenanceSpec{
				RequestorID: "some-requestor.nvidia.com",
				NodeName:    "node-1",
			},
		}
		elm2 := maintenancev1.NodeMaintenance{
			ObjectMeta: v1.ObjectMeta{Name: "bar", Namespace: "ns2"},
			Spec: maintenancev1.NodeMaintenanceSpec{
				RequestorID: "other-requestor.nvidia.com",
				NodeName:    "node-2",
			},
		}
		expected := []string{elm1.CanonicalString(), elm2.CanonicalString()}

		It("returns expected strings for list", func() {
			l := []maintenancev1.NodeMaintenance{elm1, elm2}
			strs := utils.CanonicalStringsFromList(l)
			Expect(strs).To(Equal(expected))
		})
		It("returns expected strings for pointer list", func() {
			l := []*maintenancev1.NodeMaintenance{&elm1, &elm2}
			strs := utils.CanonicalStringsFromListP(l)
			Expect(strs).To(Equal(expected))
		})
	})

	Context("ToPointerSlice", func() {
		It("behaves as expected", func() {
			src := []string{"a", "b", "c"}
			dst := utils.ToPointerSlice(src)
			for i := range src {
				Expect(*dst[i]).To(Equal(src[i]))
			}
		})
	})
})
