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

package openshift

import (
	"context"
	"fmt"

	ocpconfigv1 "github.com/openshift/api/config/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// OpenshiftUtils provides utility functions for openshift
type OpenshiftUtils interface {
	// IsOpenshift returns true if the cluster is openshift
	IsOpenshift() bool
	// IsHypershift returns true if the cluster is hypershift (openshift with extenal control plane nodes)
	IsHypershift() bool
}

// openshiftUtils implements OpenshiftUtils
type openshiftUtils struct {
	isOpenshift  bool
	isHypershift bool
}

// NewOpenshiftUtils returns a new OpenshiftUtils
func NewOpenshiftUtils(ctx context.Context, client client.Reader) (OpenshiftUtils, error) {
	ocpUtils := &openshiftUtils{}

	// check if the cluster is openshift by getting the Infrastructure resource
	infra := &ocpconfigv1.Infrastructure{}
	err := client.Get(ctx, types.NamespacedName{Name: InfraResourceName}, infra)
	if err != nil {
		if !meta.IsNoMatchError(err) {
			return nil, fmt.Errorf("can't detect cluster type, cant get infrastructure resource (name: %s): %v", InfraResourceName, err)
		}
		// not an openshift cluster
		return ocpUtils, nil
	}

	ocpUtils.isOpenshift = true
	// if control plane topology is external, it's hypershift.
	ocpUtils.isHypershift = infra.Status.ControlPlaneTopology == ocpconfigv1.ExternalTopologyMode

	return ocpUtils, nil
}

func (o *openshiftUtils) IsOpenshift() bool {
	return o.isOpenshift
}

func (o *openshiftUtils) IsHypershift() bool {
	return o.isHypershift
}
