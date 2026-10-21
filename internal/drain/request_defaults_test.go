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

package drain

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/go-logr/logr"

	maintenancev1 "github.com/Mellanox/maintenance-operator/api/v1alpha1"
)

func TestDrainRequestTimeoutDefaults(t *testing.T) {
	for _, tc := range []struct {
		name string
		spec string
		want time.Duration
	}{
		{name: "unset uses five minutes", spec: `{}`, want: 300 * time.Second},
		{name: "explicit zero is unlimited", spec: `{"timeoutSeconds":0}`, want: 0},
		{name: "explicit positive timeout", spec: `{"timeoutSeconds":10}`, want: 10 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			nm := &maintenancev1.NodeMaintenance{}
			nm.Spec.DrainSpec = &maintenancev1.DrainSpec{}
			if err := json.Unmarshal([]byte(tc.spec), nm.Spec.DrainSpec); err != nil {
				t.Fatal(err)
			}
			request := NewDrainRequest(context.Background(), logr.Discard(), nil, nm, false).(*drainRequestImpl)
			defer request.CancelDrain()
			if got := request.drainHelper.Timeout; got != tc.want {
				t.Errorf("drain helper timeout = %s, want %s", got, tc.want)
			}
		})
	}
}
