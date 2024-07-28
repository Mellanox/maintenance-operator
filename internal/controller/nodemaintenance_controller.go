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

package controller

import (
	"context"
	"sync"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	maintenancev1 "github.com/Mellanox/maintenance-operator/api/v1alpha1"
	"github.com/Mellanox/maintenance-operator/internal/k8sutils"
)

var defaultMaxNodeMaintenanceTime = 1600 * time.Second

// NewNodeMaintenanceReconcilerOptions creates new *NodeMaintenanceReconcilerOptions
func NewNodeMaintenanceReconcilerOptions() *NodeMaintenanceReconcilerOptions {
	return &NodeMaintenanceReconcilerOptions{
		pendingMaxNodeMaintenanceTime: defaultMaxNodeMaintenanceTime,
		maxNodeMaintenanceTime:        defaultMaxNodeMaintenanceTime,
	}
}

// NodeMaintenanceReconcilerOptions are options for NodeMaintenanceReconciler where values
// are stored by external entity and read by NodeMaintenanceReconciler.
type NodeMaintenanceReconcilerOptions struct {
	sync.Mutex

	pendingMaxNodeMaintenanceTime time.Duration
	maxNodeMaintenanceTime        time.Duration
}

// Store maxUnavailable, maxParallelOperations options for NodeMaintenanceReconciler
func (nmro *NodeMaintenanceReconcilerOptions) Store(maxNodeMaintenanceTime time.Duration) {
	nmro.Lock()
	defer nmro.Unlock()

	nmro.pendingMaxNodeMaintenanceTime = maxNodeMaintenanceTime
}

// Load loads the last Stored options
func (nmro *NodeMaintenanceReconcilerOptions) Load() {
	nmro.Lock()
	defer nmro.Unlock()

	nmro.maxNodeMaintenanceTime = nmro.pendingMaxNodeMaintenanceTime
}

// MaxNodeMaintenanceTime returns the last loaded MaxUnavailable option
func (nmro *NodeMaintenanceReconcilerOptions) MaxNodeMaintenanceTime() time.Duration {
	return nmro.maxNodeMaintenanceTime
}

// NodeMaintenanceReconciler reconciles a NodeMaintenance object
type NodeMaintenanceReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	EventRecorder record.EventRecorder

	Options *NodeMaintenanceReconcilerOptions
}

//+kubebuilder:rbac:groups=maintenance.nvidia.com,resources=nodemaintenances,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=maintenance.nvidia.com,resources=nodemaintenances/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=maintenance.nvidia.com,resources=nodemaintenances/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=events,verbs=create

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (r *NodeMaintenanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLog := log.FromContext(ctx)
	reqLog.Info("got request", "name", req.NamespacedName)

	// load any stored options
	r.Options.Load()
	reqLog.Info("loaded options", "maxNodeMaintenanceTime", r.Options.MaxNodeMaintenanceTime())

	// get NodeMaintenance object
	nm := &maintenancev1.NodeMaintenance{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: req.Name}, nm); err != nil {
		if k8serrors.IsNotFound(err) {
			reqLog.Info("NodeMaintenance object not found, nothing to do.")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Handle its state according to Ready Condition
	state := k8sutils.GetReadyConditionReason(nm)

	var err error
	//nolint:gocritic
	switch state {
	case maintenancev1.ConditionReasonUninitialized:
		err = r.handleUninitiaized(ctx, reqLog, nm)
		if err != nil {
			reqLog.Error(err, "failed to handle uninitialized NodeMaintenance object")
		}
	}

	return ctrl.Result{}, err
}

// handleUninitiaized handles NodeMaintenance in ConditionReasonUninitialized state
// it eventually sets NodeMaintenance Ready condition Reason to ConditionReasonPending
func (r *NodeMaintenanceReconciler) handleUninitiaized(ctx context.Context, reqLog logr.Logger, nm *maintenancev1.NodeMaintenance) error {
	reqLog.Info("Handle Uninitialized NodeMaintenance")

	// set Ready condition to ConditionReasonPending and update object
	changed := k8sutils.SetReadyConditionReason(nm, maintenancev1.ConditionReasonPending)
	var err error
	if changed {
		err = r.Status().Update(ctx, nm)
		if err != nil {
			reqLog.Error(err, "failed to update status for NodeMaintenance object")
		}
	}

	// emit state change event
	r.EventRecorder.Event(
		nm, corev1.EventTypeNormal, maintenancev1.ConditionChangedEventType, maintenancev1.ConditionReasonPending)

	return err
}

// SetupWithManager sets up the controller with the Manager.
func (r *NodeMaintenanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.EventRecorder = mgr.GetEventRecorderFor("nodemaintenancereconciler")

	return ctrl.NewControllerManagedBy(mgr).
		For(&maintenancev1.NodeMaintenance{}).
		Complete(r)
}
