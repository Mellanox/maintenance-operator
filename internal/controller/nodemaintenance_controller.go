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
	"cmp"
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	maintenancev1 "github.com/Mellanox/maintenance-operator/api/v1alpha1"
	"github.com/Mellanox/maintenance-operator/internal/cordon"
	"github.com/Mellanox/maintenance-operator/internal/drain"
	"github.com/Mellanox/maintenance-operator/internal/k8sutils"
	operatorlog "github.com/Mellanox/maintenance-operator/internal/log"
	"github.com/Mellanox/maintenance-operator/internal/podcompletion"
	"github.com/Mellanox/maintenance-operator/internal/utils"
)

var (
	waitPodCompletionRequeueTime    = 10 * time.Second
	drainReqeueTime                 = 10 * time.Second
	additionalRequestorsRequeueTime = 10 * time.Second
)

const (
	// ReadyTimeAnnotation is annotation that contains NodeMaintenance time in tranisitioned to ready state
	ReadyTimeAnnotation = "maintenance.nvidia.com/ready-time"
)

// NodeMaintenanceReconciler reconciles a NodeMaintenance object
type NodeMaintenanceReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	EventRecorder record.EventRecorder

	CordonHandler            cordon.Handler
	WaitPodCompletionHandler podcompletion.Handler
	DrainManager             drain.Manager
}

//+kubebuilder:rbac:groups=maintenance.nvidia.com,resources=nodemaintenances,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=maintenance.nvidia.com,resources=nodemaintenances/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=maintenance.nvidia.com,resources=nodemaintenances/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=events,verbs=create;update;patch
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;update;patch
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;watch;list;update;patch;delete
//+kubebuilder:rbac:groups="",resources=pods/eviction,verbs=create;get;list;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (r *NodeMaintenanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLog := log.FromContext(ctx)
	reqLog.Info("reconcile NodeMaintenance request")
	defer reqLog.Info("reconcile NodeMaintenance request end")
	reqLog.V(operatorlog.DebugLevel).Info("outstanding drain requests", "num", len(r.DrainManager.ListRequests()))

	var err error

	// get NodeMaintenance object
	nm := &maintenancev1.NodeMaintenance{}
	if err = r.Get(ctx, req.NamespacedName, nm); err != nil {
		if k8serrors.IsNotFound(err) {
			reqLog.Info("NodeMaintenance object not found, nothing to do.")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// get node object
	node := &corev1.Node{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: nm.Spec.NodeName}, node)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			reqLog.Info("node not found", "name", nm.Spec.NodeName)
			// node not found, remove finalizer from NodeMaintenance if exists
			err = k8sutils.RemoveFinalizer(ctx, r.Client, nm, maintenancev1.MaintenanceFinalizerName)
			if err != nil {
				reqLog.Error(err, "failed to remove finalizer for NodeMaintenance", "namespace", nm.Namespace, "name", nm.Name)
			}
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, err
	}

	// set owner reference if not set
	if !k8sutils.HasOwnerRef(node, nm) {
		err = k8sutils.SetOwnerRef(ctx, r.Client, node, nm)
		if err != nil {
			reqLog.Error(err, "failed to set owner reference for NodeMaintenance", "namespace", nm.Namespace, "name", nm.Name)
		}
	}

	// Handle its state according to Ready Condition
	state := k8sutils.GetReadyConditionReason(nm)
	res := ctrl.Result{}

	switch state {
	case maintenancev1.ConditionReasonUninitialized:
		err = r.handleUninitiaizedState(ctx, reqLog, nm)
		if err != nil {
			reqLog.Error(err, "failed to handle uninitialized state for NodeMaintenance object")
		}
	case maintenancev1.ConditionReasonScheduled:
		err = r.handleScheduledState(ctx, reqLog, nm)
		if err != nil {
			reqLog.Error(err, "failed to handle scheduled state for NodeMaintenance object")
		}
	case maintenancev1.ConditionReasonCordon:
		err = r.handleCordonState(ctx, reqLog, nm, node)
		if err != nil {
			reqLog.Error(err, "failed to handle cordon state for NodeMaintenance object")
		}
	case maintenancev1.ConditionReasonWaitForPodCompletion:
		res, err = r.handleWaitPodCompletionState(ctx, reqLog, nm, node)
		if err != nil {
			reqLog.Error(err, "failed to handle waitForPodCompletion state for NodeMaintenance object")
		}
	case maintenancev1.ConditionReasonDraining:
		res, err = r.handleDrainState(ctx, reqLog, nm, node)
		if err != nil {
			reqLog.Error(err, "failed to handle drain state for NodeMaintenance object")
		}
	case maintenancev1.ConditionReasonReady, maintenancev1.ConditionReasonRequestorFailed:
		res, err = r.handleTerminalState(ctx, reqLog, nm, node)
		if err != nil {
			reqLog.Error(err, "failed to handle Ready state for NodeMaintenance object")
		}
	}

	return res, err
}

// handleFinalizerRemoval handles the removal of node maintenance finalizer
func (r *NodeMaintenanceReconciler) handleFinalizerRemoval(ctx context.Context, reqLog logr.Logger, nm *maintenancev1.NodeMaintenance) error {
	reqLog.Info("removing maintenance finalizer")
	err := k8sutils.RemoveFinalizer(ctx, r.Client, nm, maintenancev1.MaintenanceFinalizerName)
	if err != nil {
		reqLog.Error(err, "failed to remove finalizer for NodeMaintenance")
	}
	return err
}

// handleUninitiaizedState handles NodeMaintenance in ConditionReasonUninitialized state
// it eventually sets NodeMaintenance Ready condition Reason to ConditionReasonPending
func (r *NodeMaintenanceReconciler) handleUninitiaizedState(ctx context.Context, reqLog logr.Logger, nm *maintenancev1.NodeMaintenance) error {
	reqLog.Info("Handle Uninitialized NodeMaintenance")

	// Set Ready condition to ConditionReasonPending and update object
	err := k8sutils.SetReadyConditionReason(ctx, r.Client, nm, maintenancev1.ConditionReasonPending)
	if err != nil {
		reqLog.Error(err, "failed to update status for NodeMaintenance object")
		return err
	}

	// emit state change event
	r.EventRecorder.Event(
		nm, corev1.EventTypeNormal, maintenancev1.ConditionChangedEventType, maintenancev1.ConditionReasonPending)

	return nil
}

// handleScheduledState handles NodeMaintenance in ConditionReasonScheduled state
// it eventually sets NodeMaintenance Ready condition Reason to ConditionReasonWaitForPodCompletion
func (r *NodeMaintenanceReconciler) handleScheduledState(ctx context.Context, reqLog logr.Logger, nm *maintenancev1.NodeMaintenance) error {
	reqLog.Info("Handle Scheduled NodeMaintenance")
	var err error

	if nm.GetDeletionTimestamp().IsZero() {
		// conditionally add finalizer
		err = k8sutils.AddFinalizer(ctx, r.Client, nm, maintenancev1.MaintenanceFinalizerName)
		if err != nil {
			reqLog.Error(err, "failed to set finalizer for NodeMaintenance")
			return err
		}
	} else {
		// object is being deleted, remove finalizer if exists and return
		reqLog.Info("NodeMaintenance object is deleting")
		return r.handleFinalizerRemoval(ctx, reqLog, nm)
	}

	// TODO(adrianc): in openshift, we should pause MCP here

	// Set Ready condition to ConditionReasonCordon and update object
	err = k8sutils.SetReadyConditionReason(ctx, r.Client, nm, maintenancev1.ConditionReasonCordon)
	if err != nil {
		reqLog.Error(err, "failed to update status for NodeMaintenance object")
		return err
	}

	// emit state change event
	r.EventRecorder.Event(
		nm, corev1.EventTypeNormal, maintenancev1.ConditionChangedEventType, maintenancev1.ConditionReasonCordon)

	return nil
}

func (r *NodeMaintenanceReconciler) handleCordonState(ctx context.Context, reqLog logr.Logger, nm *maintenancev1.NodeMaintenance, node *corev1.Node) error {
	reqLog.Info("Handle Cordon NodeMaintenance")
	var err error

	if !nm.GetDeletionTimestamp().IsZero() {
		reqLog.Info("NodeMaintenance object is deleting")

		if nm.Spec.Cordon {
			reqLog.Info("handle uncordon of node, ", "node", node.Name)
			err = r.CordonHandler.HandleUnCordon(ctx, reqLog, nm, node)
			if err != nil {
				return err
			}
		}

		// TODO(adrianc): unpause MCP in OCP when support is added.
		return r.handleFinalizerRemoval(ctx, reqLog, nm)
	}

	if nm.Spec.Cordon {
		err = r.CordonHandler.HandleCordon(ctx, reqLog, nm, node)
		if err != nil {
			return err
		}
	}

	err = k8sutils.SetReadyConditionReason(ctx, r.Client, nm, maintenancev1.ConditionReasonWaitForPodCompletion)
	if err != nil {
		reqLog.Error(err, "failed to update status for NodeMaintenance object")
		return err
	}

	r.EventRecorder.Event(
		nm, corev1.EventTypeNormal, maintenancev1.ConditionChangedEventType, maintenancev1.ConditionReasonWaitForPodCompletion)

	return nil
}

func (r *NodeMaintenanceReconciler) handleWaitPodCompletionState(ctx context.Context, reqLog logr.Logger, nm *maintenancev1.NodeMaintenance, node *corev1.Node) (ctrl.Result, error) {
	reqLog.Info("Handle WaitPodCompletion NodeMaintenance")
	// handle finalizers
	var err error
	var res ctrl.Result

	if !nm.GetDeletionTimestamp().IsZero() {
		// object is being deleted, handle cleanup.
		reqLog.Info("NodeMaintenance object is deleting")

		if nm.Spec.Cordon {
			reqLog.Info("handle uncordon of node, ", "node", node.Name)
			err = r.CordonHandler.HandleUnCordon(ctx, reqLog, nm, node)
			if err != nil {
				return res, err
			}
		}

		// TODO(adrianc): unpause MCP in OCP when support is added.
		err = r.handleFinalizerRemoval(ctx, reqLog, nm)
		return res, err
	}

	if nm.Spec.WaitForPodCompletion != nil {
		waitingForPods, err := r.WaitPodCompletionHandler.HandlePodCompletion(ctx, reqLog, nm)

		if err == nil {
			if len(waitingForPods) > 0 {
				reqLog.Info("waiting for pods to finish", "pods", waitingForPods)
				return ctrl.Result{Requeue: true, RequeueAfter: waitPodCompletionRequeueTime}, nil
			}
		} else if !errors.Is(err, podcompletion.ErrPodCompletionTimeout) {
			reqLog.Error(err, "failed to handle waitPodCompletion")
			return res, err
		}
		// Note(adrianc): we get here if waitingForPods is zero length or timeout reached, in any case
		// we can can progress to next step for this NodeMaintenance
	}

	// update condition and send event
	err = k8sutils.SetReadyConditionReason(ctx, r.Client, nm, maintenancev1.ConditionReasonDraining)
	if err != nil {
		reqLog.Error(err, "failed to update status for NodeMaintenance object")
		return res, err
	}

	r.EventRecorder.Event(
		nm, corev1.EventTypeNormal, maintenancev1.ConditionChangedEventType, maintenancev1.ConditionReasonDraining)

	return res, nil
}

func (r *NodeMaintenanceReconciler) handleDrainState(ctx context.Context, reqLog logr.Logger, nm *maintenancev1.NodeMaintenance, node *corev1.Node) (ctrl.Result, error) {
	reqLog.Info("Handle Draining NodeMaintenance")
	var err error
	var res ctrl.Result

	if !nm.GetDeletionTimestamp().IsZero() {
		// object is being deleted, handle cleanup.
		reqLog.Info("NodeMaintenance object is deleting")

		reqLog.Info("handle drain request removal")
		drainReqUID := drain.DrainRequestUIDFromNodeMaintenance(nm)
		req := r.DrainManager.GetRequest(drainReqUID)
		if req != nil {
			reqLog.Info("stopping and removing drain request", "reqUID", drainReqUID, "state", req.State())
		}
		r.DrainManager.RemoveRequest(drainReqUID)

		if nm.Spec.Cordon {
			reqLog.Info("handle uncordon of node, ", "node", node.Name)
			err = r.CordonHandler.HandleUnCordon(ctx, reqLog, nm, node)
			if err != nil {
				return res, err
			}
		}

		// TODO(adrianc): unpause MCP in OCP when support is added.

		// remove finalizer if exists and return
		reqLog.Info("NodeMaintenance object is deleting, removing maintenance finalizer")
		err = r.handleFinalizerRemoval(ctx, reqLog, nm)
		return res, err
	}

	if nm.Spec.DrainSpec != nil {
		drainReqUID := drain.DrainRequestUIDFromNodeMaintenance(nm)
		req := r.DrainManager.GetRequest(drainReqUID)
		if req == nil {
			reqLog.Info("sending new drain request")
			req = r.DrainManager.NewDrainRequest(nm)
			// reset and update initial drain status
			nm.Status.Drain = nil
			if err = r.updateDrainStatus(ctx, nm, req); err != nil {
				return res, err
			}
			_ = r.DrainManager.AddRequest(req)
			return ctrl.Result{Requeue: true, RequeueAfter: drainReqeueTime}, nil
		}

		reqLog.Info("drain request details", "uid", req.UID(), "state", req.State())

		// handle update of drain spec
		if !reflect.DeepEqual(req.Spec().Spec, *nm.Spec.DrainSpec) {
			reqLog.Info("drain spec has changed, removing current request and requeue")
			r.DrainManager.RemoveRequest(req.UID())
			return ctrl.Result{Requeue: true, RequeueAfter: drainReqeueTime}, nil
		}

		if req.State() == drain.DrainStateInProgress {
			// update progress and requeue
			if err = r.updateDrainStatus(ctx, nm, req); err != nil {
				return res, err
			}
			return ctrl.Result{Requeue: true, RequeueAfter: drainReqeueTime}, nil
		}

		// handle request in error state
		if req.State() == drain.DrainStateError || req.State() == drain.DrainStateCanceled {
			reqLog.Info("drain request error. removing current request and requeue", "state", req.State())
			r.DrainManager.RemoveRequest(req.UID())
			return ctrl.Result{Requeue: true, RequeueAfter: drainReqeueTime}, nil
		}

		// Drain completed successfully
		reqLog.Info("drain completed successfully", "reqUID", req.UID(), "state", req.State())
		if err = r.updateDrainStatus(ctx, nm, req); err != nil {
			return res, err
		}

		r.DrainManager.RemoveRequest(req.UID())
	} else {
		// if nil, remove any pending requests for NodeMaintenance if present
		r.DrainManager.RemoveRequest(drain.DrainRequestUIDFromNodeMaintenance(nm))
		if nm.Status.Drain != nil {
			// clear out drain status
			nm.Status.Drain = nil
			err = r.Client.Status().Update(ctx, nm)
			if err != nil {
				reqLog.Error(err, "failed to update drain status for NodeMaintenance object")
				return res, err
			}
		}
	}

	// update condition and send event
	err = k8sutils.SetReadyConditionReason(ctx, r.Client, nm, maintenancev1.ConditionReasonReady)
	if err != nil {
		reqLog.Error(err, "failed to update status for NodeMaintenance object")
		return res, err
	}

	r.EventRecorder.Event(
		nm, corev1.EventTypeNormal, maintenancev1.ConditionChangedEventType, maintenancev1.ConditionReasonReady)

	return res, nil
}

// updateDrainStatus updates NodeMaintenance drain status in place. returns error if occurred
func (r *NodeMaintenanceReconciler) updateDrainStatus(ctx context.Context, nm *maintenancev1.NodeMaintenance, drainReq drain.DrainRequest) error {
	ds, err := drainReq.Status()
	if err != nil {
		return fmt.Errorf("failed to update drain status. %w", err)
	}

	if nm.Status.Drain == nil {
		// set initial status
		podsOnNode := &corev1.PodList{}
		selectorFields := fields.OneTermEqualSelector("spec.nodeName", nm.Spec.NodeName)
		err = r.Client.List(ctx, podsOnNode, &client.ListOptions{FieldSelector: selectorFields})
		if err != nil {
			return fmt.Errorf("failed to list pods. %w", err)
		}

		nm.Status.Drain = &maintenancev1.DrainStatus{
			TotalPods:    int32(len(podsOnNode.Items)),
			EvictionPods: int32(len(ds.PodsToDelete)),
		}
	}

	removedPods := utils.Max(int(nm.Status.Drain.EvictionPods)-len(ds.PodsToDelete), 0)

	nm.Status.Drain.DrainProgress = 100
	if nm.Status.Drain.EvictionPods != 0 {
		nm.Status.Drain.DrainProgress = int32(float32(removedPods) / float32(nm.Status.Drain.EvictionPods) * 100)
	}
	nm.Status.Drain.WaitForEviction = ds.PodsToDelete

	err = r.Client.Status().Update(ctx, nm)
	if err != nil {
		return fmt.Errorf("failed to update drain status for NodeMaintenance object. %w", err)
	}

	return nil
}

func (r *NodeMaintenanceReconciler) handleTerminalState(ctx context.Context, reqLog logr.Logger, nm *maintenancev1.NodeMaintenance, node *corev1.Node) (ctrl.Result, error) {
	reqLog.Info("Handle Ready/RequestorFailed NodeMaintenance")
	var err error
	var res ctrl.Result

	if !nm.GetDeletionTimestamp().IsZero() {
		// object is being deleted, handle cleanup.
		reqLog.Info("NodeMaintenance object is deleting")

		if len(nm.Spec.AdditionalRequestors) > 0 {
			reqLog.Info("additional requestors for node maintenance. waiting for list to clear, requeue request", "additionalRequestors", nm.Spec.AdditionalRequestors)
			return ctrl.Result{Requeue: true, RequeueAfter: additionalRequestorsRequeueTime}, nil
		}

		if nm.Spec.Cordon {
			reqLog.Info("handle uncordon of node", "node", node.Name)
			err = r.CordonHandler.HandleUnCordon(ctx, reqLog, nm, node)
			if err != nil {
				return res, err
			}
		}

		// TODO(adrianc): unpause MCP in OCP when support is added.

		// remove finalizer if exists and return
		err = r.handleFinalizerRemoval(ctx, reqLog, nm)
		return res, err
	}

	// set ready-time annotation if not present on not set
	if nm.Annotations[ReadyTimeAnnotation] == "" {
		metav1.SetMetaDataAnnotation(&nm.ObjectMeta, ReadyTimeAnnotation, time.Now().UTC().Format(time.RFC3339))
		err = r.Update(ctx, nm)
		if err != nil {
			return res, err
		}
	}

	// check RequestorFailed condition
	currentReason := k8sutils.GetReadyConditionReason(nm)
	conditionChanged := false
	cond := meta.FindStatusCondition(nm.Status.Conditions, maintenancev1.ConditionTypeRequestorFailed)
	if cond != nil && cond.Status == metav1.ConditionTrue {
		// set Ready condition to RequestorFailed reason if not already set
		if currentReason == maintenancev1.ConditionReasonReady {
			err = k8sutils.SetReadyConditionReason(ctx, r.Client, nm, maintenancev1.ConditionReasonRequestorFailed)
			conditionChanged = true
		}
	} else {
		// set Ready condition to Ready reason if not already set
		if currentReason == maintenancev1.ConditionReasonRequestorFailed {
			err = k8sutils.SetReadyConditionReason(ctx, r.Client, nm, maintenancev1.ConditionReasonReady)
			conditionChanged = true
		}
	}
	if err != nil {
		return res, err
	}

	if conditionChanged {
		r.EventRecorder.Event(
			nm, corev1.EventTypeNormal, maintenancev1.ConditionChangedEventType, k8sutils.GetReadyConditionReason(nm))
	}

	return res, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NodeMaintenanceReconciler) SetupWithManager(mgr ctrl.Manager, log logr.Logger) error {
	r.EventRecorder = mgr.GetEventRecorderFor("nodemaintenancereconciler")

	return ctrl.NewControllerManagedBy(mgr).
		For(&maintenancev1.NodeMaintenance{}, builder.WithPredicates(NewConditionChangedPredicate(log))).
		Complete(r)
}

// NewConditionChangedPredicate creates a new ConditionChangedPredicate
func NewConditionChangedPredicate(log logr.Logger) ConditionChangedPredicate {
	return ConditionChangedPredicate{
		Funcs: predicate.Funcs{},
		log:   log,
	}
}

// ConditionChangedPredicate will trigger enqueue of Event for reconcile in the following cases:
// 1. A change in NodeMaintenance Conditions
// 2. Update to the object occurred and deletion timestamp is set
// 3. NodeMaintenance created
// 4. NodeMaintenance deleted
// 5. generic event received
type ConditionChangedPredicate struct {
	predicate.Funcs

	log logr.Logger
}

// Update implements Predicate.
func (p ConditionChangedPredicate) Update(e event.TypedUpdateEvent[client.Object]) bool {
	p.log.V(operatorlog.DebugLevel).Info("ConditionChangedPredicate Update")

	if e.ObjectOld == nil {
		p.log.Error(nil, "old object is nil in update event, ignoring event.")
		return false
	}
	if e.ObjectNew == nil {
		p.log.Error(nil, "new object is nil in update event, ignoring event.")
		return false
	}

	oldO, ok := e.ObjectOld.(*maintenancev1.NodeMaintenance)
	if !ok {
		p.log.Error(nil, "failed to cast old object to NodeMaintenance in update event, ignoring event.")
		return false
	}

	newO, ok := e.ObjectNew.(*maintenancev1.NodeMaintenance)
	if !ok {
		p.log.Error(nil, "failed to cast new object to NodeMaintenance in update event, ignoring event.")
		return false
	}

	cmpByType := func(a, b metav1.Condition) int {
		return cmp.Compare(a.Type, b.Type)
	}

	// sort old and new obj.Status.Conditions so they can be compared using DeepEqual
	slices.SortFunc(oldO.Status.Conditions, cmpByType)
	slices.SortFunc(newO.Status.Conditions, cmpByType)

	condChanged := !reflect.DeepEqual(oldO.Status.Conditions, newO.Status.Conditions)
	deleting := !newO.GetDeletionTimestamp().IsZero()
	enqueue := condChanged || deleting

	p.log.V(operatorlog.DebugLevel).Info("update event for NodeMaintenance",
		"name", newO.Name, "namespace", newO.Namespace,
		"condition-changed", condChanged,
		"deleting", deleting, "enqueue-request", enqueue)

	return enqueue
}
