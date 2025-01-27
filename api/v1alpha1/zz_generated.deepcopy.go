//go:build !ignore_autogenerated

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

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DrainSpec) DeepCopyInto(out *DrainSpec) {
	*out = *in
	if in.PodEvictionFilters != nil {
		in, out := &in.PodEvictionFilters, &out.PodEvictionFilters
		*out = make([]PodEvictionFiterEntry, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DrainSpec.
func (in *DrainSpec) DeepCopy() *DrainSpec {
	if in == nil {
		return nil
	}
	out := new(DrainSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DrainStatus) DeepCopyInto(out *DrainStatus) {
	*out = *in
	if in.WaitForEviction != nil {
		in, out := &in.WaitForEviction, &out.WaitForEviction
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DrainStatus.
func (in *DrainStatus) DeepCopy() *DrainStatus {
	if in == nil {
		return nil
	}
	out := new(DrainStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MaintenanceOperatorConfig) DeepCopyInto(out *MaintenanceOperatorConfig) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MaintenanceOperatorConfig.
func (in *MaintenanceOperatorConfig) DeepCopy() *MaintenanceOperatorConfig {
	if in == nil {
		return nil
	}
	out := new(MaintenanceOperatorConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *MaintenanceOperatorConfig) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MaintenanceOperatorConfigList) DeepCopyInto(out *MaintenanceOperatorConfigList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]MaintenanceOperatorConfig, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MaintenanceOperatorConfigList.
func (in *MaintenanceOperatorConfigList) DeepCopy() *MaintenanceOperatorConfigList {
	if in == nil {
		return nil
	}
	out := new(MaintenanceOperatorConfigList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *MaintenanceOperatorConfigList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MaintenanceOperatorConfigSpec) DeepCopyInto(out *MaintenanceOperatorConfigSpec) {
	*out = *in
	if in.MaxParallelOperations != nil {
		in, out := &in.MaxParallelOperations, &out.MaxParallelOperations
		*out = new(intstr.IntOrString)
		**out = **in
	}
	if in.MaxUnavailable != nil {
		in, out := &in.MaxUnavailable, &out.MaxUnavailable
		*out = new(intstr.IntOrString)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MaintenanceOperatorConfigSpec.
func (in *MaintenanceOperatorConfigSpec) DeepCopy() *MaintenanceOperatorConfigSpec {
	if in == nil {
		return nil
	}
	out := new(MaintenanceOperatorConfigSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MaintenanceOperatorConfigStatus) DeepCopyInto(out *MaintenanceOperatorConfigStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MaintenanceOperatorConfigStatus.
func (in *MaintenanceOperatorConfigStatus) DeepCopy() *MaintenanceOperatorConfigStatus {
	if in == nil {
		return nil
	}
	out := new(MaintenanceOperatorConfigStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodeMaintenance) DeepCopyInto(out *NodeMaintenance) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NodeMaintenance.
func (in *NodeMaintenance) DeepCopy() *NodeMaintenance {
	if in == nil {
		return nil
	}
	out := new(NodeMaintenance)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NodeMaintenance) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodeMaintenanceList) DeepCopyInto(out *NodeMaintenanceList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]NodeMaintenance, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NodeMaintenanceList.
func (in *NodeMaintenanceList) DeepCopy() *NodeMaintenanceList {
	if in == nil {
		return nil
	}
	out := new(NodeMaintenanceList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NodeMaintenanceList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodeMaintenanceSpec) DeepCopyInto(out *NodeMaintenanceSpec) {
	*out = *in
	if in.AdditionalRequestors != nil {
		in, out := &in.AdditionalRequestors, &out.AdditionalRequestors
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.WaitForPodCompletion != nil {
		in, out := &in.WaitForPodCompletion, &out.WaitForPodCompletion
		*out = new(WaitForPodCompletionSpec)
		**out = **in
	}
	if in.DrainSpec != nil {
		in, out := &in.DrainSpec, &out.DrainSpec
		*out = new(DrainSpec)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NodeMaintenanceSpec.
func (in *NodeMaintenanceSpec) DeepCopy() *NodeMaintenanceSpec {
	if in == nil {
		return nil
	}
	out := new(NodeMaintenanceSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodeMaintenanceStatus) DeepCopyInto(out *NodeMaintenanceStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.WaitForCompletion != nil {
		in, out := &in.WaitForCompletion, &out.WaitForCompletion
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Drain != nil {
		in, out := &in.Drain, &out.Drain
		*out = new(DrainStatus)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NodeMaintenanceStatus.
func (in *NodeMaintenanceStatus) DeepCopy() *NodeMaintenanceStatus {
	if in == nil {
		return nil
	}
	out := new(NodeMaintenanceStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PodEvictionFiterEntry) DeepCopyInto(out *PodEvictionFiterEntry) {
	*out = *in
	if in.ByResourceNameRegex != nil {
		in, out := &in.ByResourceNameRegex, &out.ByResourceNameRegex
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PodEvictionFiterEntry.
func (in *PodEvictionFiterEntry) DeepCopy() *PodEvictionFiterEntry {
	if in == nil {
		return nil
	}
	out := new(PodEvictionFiterEntry)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WaitForPodCompletionSpec) DeepCopyInto(out *WaitForPodCompletionSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WaitForPodCompletionSpec.
func (in *WaitForPodCompletionSpec) DeepCopy() *WaitForPodCompletionSpec {
	if in == nil {
		return nil
	}
	out := new(WaitForPodCompletionSpec)
	in.DeepCopyInto(out)
	return out
}
