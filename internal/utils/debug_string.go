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

package utils

import (
	maintenancev1 "github.com/Mellanox/maintenance-operator/api/v1alpha1"
)

// CanonicalStringsFromListP returns list with the CanonicalString value of NodeMaintenance
func CanonicalStringsFromListP(l []*maintenancev1.NodeMaintenance) []string {
	sl := make([]string, 0, len(l))
	for _, nm := range l {
		sl = append(sl, nm.CanonicalString())
	}

	return sl
}

// CanonicalStringsFromList returns list with the CanonicalString value of NodeMaintenance
func CanonicalStringsFromList(l []maintenancev1.NodeMaintenance) []string {
	sl := make([]string, 0, len(l))
	for _, nm := range l {
		sl = append(sl, nm.CanonicalString())
	}

	return sl
}
