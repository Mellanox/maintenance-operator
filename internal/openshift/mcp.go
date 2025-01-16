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
	"errors"
	"fmt"
	"sync"
	"time"

	mcv1 "github.com/openshift/api/machineconfiguration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	maintenancev1 "github.com/Mellanox/maintenance-operator/api/v1alpha1"
)

const (
	// InfraResourceName is the name of the openshift infrastructure resource
	InfraResourceName = "cluster"
	// DesiredMachineConfigAnnotationKey is used to specify the desired MachineConfig for a machine
	DesiredMachineConfigAnnotationKey = "machineconfiguration.openshift.io/desiredConfig"

	// McpPausedAnnotKey is the annotations used to mark a MachineConfigPool as paused by the operator
	McpPausedAnnotKey   = "maintenance.nvidia.com/mcp-paused"
	McpPausedAnnotValue = "true"

	// McpNameLabelKey is the label that contains the name of the MachineConfigPool which was paused. it is set on NodeMaintenance object
	McpNameLabelKey = "maintenance.nvidia.com/paused-mcp-name"
)

// ErrMachineConfigBusy is returned when MachineConfigPool is busy, either currently under configuration or is paused by another entity
// operation should be retried at a later time
var ErrMachineConfigBusy = errors.New("machineconfigpool busy")

// MCPManager manages MachineConfigPool operations
type MCPManager interface {
	// PauseMCP pauses the MachineConfigPool on the given node
	PauseMCP(ctx context.Context, node *corev1.Node, nm *maintenancev1.NodeMaintenance) error
	// UnpauseMCP unpauses the MachineConfigPool on the given node
	UnpauseMCP(ctx context.Context, node *corev1.Node, nm *maintenancev1.NodeMaintenance) error
}

// NewMCPManager returns a new MCPManager based on the cluster type
func NewMCPManager(ocpUtils OpenshiftUtils, client client.Client) MCPManager {
	if ocpUtils.IsOpenshift() && !ocpUtils.IsHypershift() {
		return NewOpenshiftMcpManager(client)
	}
	return NewNoOpMcpManager()
}

// noOpMcpManager implements MCPManager with no-op methods
type noOpMcpManager struct{}

func (n *noOpMcpManager) PauseMCP(ctx context.Context, node *corev1.Node, nm *maintenancev1.NodeMaintenance) error {
	return nil
}

func (n *noOpMcpManager) UnpauseMCP(ctx context.Context, node *corev1.Node, nm *maintenancev1.NodeMaintenance) error {
	return nil
}

// NewNoOpMcpManager returns a no-op MCPManager. used for non-openshift clusters
func NewNoOpMcpManager() MCPManager {
	return &noOpMcpManager{}
}

// openshiftMcpManager implements MCPManager for openshift clusters
type openshiftMcpManager struct {
	client client.Client
	mu     sync.Mutex
}

// NewOpenshiftMcpManager returns a new MCPManager. used for openshift clusters
func NewOpenshiftMcpManager(client client.Client) MCPManager {
	return &openshiftMcpManager{client: client, mu: sync.Mutex{}}
}

func (o *openshiftMcpManager) PauseMCP(ctx context.Context, node *corev1.Node, nm *maintenancev1.NodeMaintenance) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	mcpName, err := o.getMCPName(ctx, node)
	if err != nil {
		return fmt.Errorf("failed to pause MachineConfigPool. failed to get pool for node %s: %w", node.Name, err)
	}

	// set mcp name annotation on NodeMaintenance
	if nm.Labels[McpNameLabelKey] != mcpName {
		metav1.SetMetaDataLabel(&nm.ObjectMeta, McpNameLabelKey, mcpName)
		err = o.client.Update(ctx, nm)
		if err != nil {
			return fmt.Errorf("failed to pause MachineConfigPool. failed to set MachineConfigPool name annotation on NodeMaintenance %s: %w", nm.Name, err)
		}

		// wait for mcp name label of nodeMaintenance to be reflected back in cache sice we depend on this label for MCP unpause (of other NodeMaintenances)
		err = waitForObjectUpdate(ctx, o.client, nm, func(obj client.Object) bool {
			return obj.GetLabels()[McpNameLabelKey] == mcpName
		})
		if err != nil {
			return fmt.Errorf("failed to pause MachineConfigPool %s. failed while waiting for NodeMaintenance label to update: %w", mcpName, err)
		}
	}

	mcp := &mcv1.MachineConfigPool{}
	err = o.client.Get(ctx, client.ObjectKey{Name: mcpName}, mcp)
	if err != nil {
		return fmt.Errorf("failed to pause MachineConfigPool. failed to get MachineConfigPool %s: %w", mcpName, err)
	}

	// if already paused by the operator return
	if mcp.Annotations[McpPausedAnnotKey] == McpPausedAnnotValue {
		return nil
	}

	// if paused, but not by the operator return with error ErrMachineConfigBusy
	if mcp.Spec.Paused && !metav1.HasAnnotation(mcp.ObjectMeta, McpPausedAnnotKey) {
		return fmt.Errorf("failed to pause MachineConfigPool. %s is already paused by another entity. %w", mcpName, ErrMachineConfigBusy)
	}

	// check if machine config is doing something, if so return error ErrMachineConfigBusy
	if mcp.Spec.Configuration.Name != mcp.Status.Configuration.Name {
		return fmt.Errorf("failed to pause MachineConfigPool. %s is in the middle of a configuration change. %w", mcpName, ErrMachineConfigBusy)
	}

	// pause the MachineConfigPool
	err = o.changeMachineConfigPoolPause(ctx, mcp, true)
	if err != nil {
		return fmt.Errorf("failed to pause MachineConfigPool %s: %w", mcpName, err)
	}

	// wait for pause annotation to be reflected back in cache
	err = waitForObjectUpdate(ctx, o.client, mcp, func(obj client.Object) bool {
		return obj.GetAnnotations()[McpPausedAnnotKey] == McpPausedAnnotValue
	})
	if err != nil {
		return fmt.Errorf("failed to pause MachineConfigPool %s. failed while waiting for annotation to update: %w", mcpName, err)
	}

	// check if machine config is started doing something, if so, undo operation and return error
	if mcp.Spec.Configuration.Name != mcp.Status.Configuration.Name {
		err = o.changeMachineConfigPoolPause(ctx, mcp, false)
		if err != nil {
			return fmt.Errorf("failed to pause MachineConfigPool.%s is configuring, failed to unpause it: %w", mcpName, err)
		}
		// wait for pause annotation removal to be reflected back in cache
		err = waitForObjectUpdate(ctx, o.client, mcp, func(obj client.Object) bool {
			_, ok := obj.GetAnnotations()[McpPausedAnnotKey]
			return !ok
		})
		if err != nil {
			return fmt.Errorf("failed to pause MachineConfigPool %s. failed while waiting for annotation to be removed: %w", mcpName, err)
		}

		return fmt.Errorf("failed to pause MachineConfigPool. %s is in the middle of a configuration change. %w", mcpName, ErrMachineConfigBusy)
	}

	return nil
}

func (o *openshiftMcpManager) UnpauseMCP(ctx context.Context, node *corev1.Node, nm *maintenancev1.NodeMaintenance) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	// get MCP
	mcpName, err := o.getMCPName(ctx, node)
	if err != nil {
		return fmt.Errorf("failed to unpause MachineConfigPool. failed to get pool for node %s: %w", node.Name, err)
	}

	mcp := &mcv1.MachineConfigPool{}
	err = o.client.Get(ctx, client.ObjectKey{Name: mcpName}, mcp)
	if err != nil {
		return fmt.Errorf("failed to unpause MachineConfigPool. failed to get MachineConfigPool %s: %w", mcpName, err)
	}

	// if mcp not paused by operator, return.
	// this can happen if node changes MCP while under maintenance.
	if !metav1.HasAnnotation(mcp.ObjectMeta, McpPausedAnnotKey) {
		return nil
	}

	// check if MCP is used by other NodeMaintenance
	used, err := o.mcpUsedByOtherNodeMaintenance(ctx, mcp, nm)
	if err != nil {
		return fmt.Errorf("failed to unpause MachineConfigPool %s, failed to check if it is used by other nodeMaintenances: %w", mcpName, err)
	}

	// if not used, we can unpause MCP
	if !used {
		err = o.changeMachineConfigPoolPause(ctx, mcp, false)
		if err != nil {
			return fmt.Errorf("failed to unpause MachineConfigPool %s: %w", mcpName, err)
		}

		// wait for unpause annotation to be reflected back in cache
		err = waitForObjectUpdate(ctx, o.client, mcp, func(obj client.Object) bool {
			_, ok := obj.GetAnnotations()[McpPausedAnnotKey]
			return !ok
		})
		if err != nil {
			return fmt.Errorf("failed to unpause MachineConfigPool %s. failed while waiting for annotation to be removed: %w", mcpName, err)
		}
	}
	return nil
}

// getMCPName returns the MachineConfigPool name for the given node
func (o *openshiftMcpManager) getMCPName(ctx context.Context, node *corev1.Node) (string, error) {
	// To get the MachineConfigPool for the node, we do the following:
	// 1. get the desired MachineConfig from the node annotation
	// 2. find the owning MachineConfigPool for the MachineConfig we found in step 1
	desiredConfig, ok := node.Annotations[DesiredMachineConfigAnnotationKey]
	if !ok {
		return "", fmt.Errorf("failed to get MachineConfig, desired machine config annotation for node %s not found", node.Name)
	}

	mc := &mcv1.MachineConfig{}
	err := o.client.Get(ctx, client.ObjectKey{Name: desiredConfig}, mc)
	if err != nil {
		return "", fmt.Errorf("failed to get the the node's MachineConfig object with name %s. %w", desiredConfig, err)
	}
	for _, owner := range mc.OwnerReferences {
		if owner.Kind == "MachineConfigPool" {
			return owner.Name, nil
		}
	}
	return "", fmt.Errorf("failed to find the MachineConfigPool of the node, no owner for the node's MachineConfig")
}

// changeMachineConfigPoolPause pauses/unpauses the MachineConfigPool
func (o *openshiftMcpManager) changeMachineConfigPoolPause(ctx context.Context, mcp *mcv1.MachineConfigPool, pause bool) error {
	// create merge patch to pause/unpause and annotate/un-annotate the machine config pool
	var patch []byte
	if pause {
		patch = []byte(fmt.Sprintf(`{"spec":{"paused":true},"metadata":{"annotations":{"%s":"%s"}}}`, McpPausedAnnotKey, McpPausedAnnotValue))
	} else {
		patch = []byte(fmt.Sprintf(`{"spec":{"paused": false},"metadata":{"annotations":{"%s": null}}}`, McpPausedAnnotKey))
	}

	err := o.client.Patch(ctx, mcp, client.RawPatch(types.MergePatchType, patch))
	if err != nil {
		return fmt.Errorf("failed to patch MachineConfigPool %s to pause=%t: %w", mcp.Name, pause, err)
	}

	return nil
}

// mcpUsedByOtherNodeMaintenance return true if the MCP is used by other Nodes which have NodeMaintenance associated with them that already paused MCP
func (o *openshiftMcpManager) mcpUsedByOtherNodeMaintenance(ctx context.Context, mcp *mcv1.MachineConfigPool, nm *maintenancev1.NodeMaintenance) (bool, error) {
	// get all nodes that match the pool
	nodesInPool := &corev1.NodeList{}
	selector, err := metav1.LabelSelectorAsSelector(mcp.Spec.NodeSelector)
	if err != nil {
		return false, err
	}

	err = o.client.List(ctx, nodesInPool, &client.ListOptions{LabelSelector: selector})
	if err != nil {
		return false, err
	}

	nodesInPoolMap := make(map[string]bool)
	for _, n := range nodesInPool.Items {
		nodesInPoolMap[n.Name] = true
	}

	nml := &maintenancev1.NodeMaintenanceList{}
	err = o.client.List(ctx, nml)
	if err != nil {
		return false, err
	}

	// check that no other NodeMaintenance is using the MCP
	for _, nmItem := range nml.Items {
		// if nodeMaintenance is not in the pool, skip
		if !nodesInPoolMap[nmItem.Spec.NodeName] {
			continue
		}
		// if MCP name label is not set on NodeMaintenance, skip (PauseMCP was not called for this NodeMaintenance)
		if nmItem.Labels[McpNameLabelKey] == "" {
			continue
		}

		// if its the current nodeMaintenance, skip
		if nmItem.Namespace == nm.Namespace &&
			nmItem.Name == nm.Name {
			continue
		}

		// mcp is used by other NodeMaintenance
		return true, nil
	}

	return false, nil
}

// waitForObjectUpdate polls k8s using the provided client, gets the given object and checks if the object satisfies the checkFn
// checkFn should return true if the object satisfies the condition, false otherwise.
// this is useful when we need to wait for an object to be updated in the client cache
func waitForObjectUpdate(ctx context.Context, kclient client.Client, obj client.Object, checkFn func(o client.Object) bool) error {
	return wait.PollUntilContextTimeout(ctx, 250*time.Millisecond, 10*time.Second, true, func(ctx context.Context) (bool, error) {
		err := kclient.Get(ctx, client.ObjectKeyFromObject(obj), obj)
		if err != nil {
			return false, err
		}

		return checkFn(obj), nil
	})
}
