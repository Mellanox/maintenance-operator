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
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	maintenancev1 "github.com/Mellanox/maintenance-operator/api/v1alpha1"
	"github.com/Mellanox/maintenance-operator/internal/k8sutils"
	operatorlog "github.com/Mellanox/maintenance-operator/internal/log"
	"github.com/Mellanox/maintenance-operator/internal/scheduler"
	"github.com/Mellanox/maintenance-operator/internal/utils"
)

const (
	schedulerSyncEventName = "node-maintenance-scheduler-sync-event"
	schedulerSyncTime      = 10 * time.Second
)

var defaultmaxParallelOperations = &intstr.IntOrString{Type: intstr.Int, IntVal: 1}

var schedulerResyncResult = ctrl.Result{RequeueAfter: schedulerSyncTime}

// NewNodeMaintenanceSchedulerReconcilerOptions creates new *NodeMaintenanceSchedulerReconcilerOptions
func NewNodeMaintenanceSchedulerReconcilerOptions() *NodeMaintenanceSchedulerReconcilerOptions {
	return &NodeMaintenanceSchedulerReconcilerOptions{
		Mutex:                        sync.Mutex{},
		pendingMaxUnavailable:        nil,
		pendingMaxParallelOperations: defaultmaxParallelOperations,
		maxUnavailable:               nil,
		maxParallelOperations:        defaultmaxParallelOperations,
	}
}

// NodeMaintenanceSchedulerReconcilerOptions are options for NodeMaintenanceSchedulerReconciler where values
// are stored by external entity and read by NodeMaintenanceSchedulerReconciler.
type NodeMaintenanceSchedulerReconcilerOptions struct {
	sync.Mutex

	pendingMaxUnavailable        *intstr.IntOrString
	pendingMaxParallelOperations *intstr.IntOrString
	maxUnavailable               *intstr.IntOrString
	maxParallelOperations        *intstr.IntOrString
}

// Store maxUnavailable, maxParallelOperations options for NodeMaintenanceSchedulerReconciler
func (nmsro *NodeMaintenanceSchedulerReconcilerOptions) Store(maxUnavailable, maxParallelOperations *intstr.IntOrString) {
	nmsro.Lock()
	defer nmsro.Unlock()

	nmsro.pendingMaxUnavailable = maxUnavailable
	nmsro.pendingMaxParallelOperations = maxParallelOperations
}

// Load loads the last Stored options
func (nmsro *NodeMaintenanceSchedulerReconcilerOptions) Load() {
	nmsro.Lock()
	defer nmsro.Unlock()

	nmsro.maxUnavailable = nmsro.pendingMaxUnavailable
	nmsro.maxParallelOperations = nmsro.pendingMaxParallelOperations
}

// MaxUnavailable returns the last loaded MaxUnavailable option
func (nmsro *NodeMaintenanceSchedulerReconcilerOptions) MaxUnavailable() *intstr.IntOrString {
	return nmsro.maxUnavailable
}

// MaxParallelOperations returns the last loaded MaxParallelOperations option
func (nmsro *NodeMaintenanceSchedulerReconcilerOptions) MaxParallelOperations() *intstr.IntOrString {
	return nmsro.maxParallelOperations
}

// NodeMaintenanceSchedulerReconciler reconciles a NodeMaintenance object
type NodeMaintenanceSchedulerReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	EventRecorder record.EventRecorder

	Options *NodeMaintenanceSchedulerReconcilerOptions
	Log     logr.Logger
	Sched   scheduler.Scheduler
}

//+kubebuilder:rbac:groups=maintenance.nvidia.com,resources=nodemaintenances,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=maintenance.nvidia.com,resources=nodemaintenances/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=maintenance.nvidia.com,resources=nodemaintenances/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=events,verbs=create

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (r *NodeMaintenanceSchedulerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Log.Info("run periodic scheduler loop")
	defer r.Log.Info("run periodic scheduler loop end")
	// load any stored options
	r.Options.Load()
	r.Log.Info("loaded options", "maxUnavailable",
		r.Options.MaxUnavailable(), "maxParallelOperations", r.Options.MaxParallelOperations())

	// Check if we can schedule new NodeMaintenance requests, determine how many maintenance slots we have
	nl := &corev1.NodeList{}
	err := r.List(ctx, nl)
	if err != nil {
		r.Log.Error(err, "failed to list Nodes")
		return ctrl.Result{}, err
	}
	nodes := utils.ToPointerSlice(nl.Items)

	nml := &maintenancev1.NodeMaintenanceList{}
	err = r.List(ctx, nml)
	if err != nil {
		r.Log.Error(err, "failed to list NodeMaintenance")
		return ctrl.Result{}, err
	}
	nms := utils.ToPointerSlice(nml.Items)

	schedCtx, err := r.preSchedule(nodes, nms)
	if err != nil {
		r.Log.Error(err, "failed preSchedule")
		return ctrl.Result{}, err
	}

	if len(schedCtx.CandidateMaintenance) == 0 {
		r.Log.Info("no candidate NodeMaintenance requests")
		return schedulerResyncResult, nil
	}

	if len(schedCtx.CandidateNodes) == 0 {
		r.Log.Info("no candidate nodes available for maintenance scheduling")
		return schedulerResyncResult, nil
	}

	if schedCtx.AvailableSlots == 0 {
		r.Log.Info("no slots available for maintenance scheduling")
		return schedulerResyncResult, nil
	}

	// Construct ClusterState
	clusterState := scheduler.NewClusterState(nodes, nms)

	// invoke Scheduler to nominate NodeMaintenance requests for maintenance
	toSchedule := r.Sched.Schedule(clusterState, schedCtx)

	// set NodeMaintenance Ready Condition Reason to Scheduled
	wg := sync.WaitGroup{}
	for _, nm := range toSchedule {
		nm := nm
		wg.Add(1)
		go func() {
			defer wg.Done()
			// update status
			err := k8sutils.SetReadyConditionReason(ctx, r.Client, nm, maintenancev1.ConditionReasonScheduled)
			if err != nil {
				r.Log.Error(err, "failed to update condition for NodeMaintenance", "name", nm.Name, "namespace", nm.Namespace)
				return
			}

			// emit event
			r.EventRecorder.Event(nm, corev1.EventTypeNormal, maintenancev1.ConditionChangedEventType, maintenancev1.ConditionReasonScheduled)

			// wait for condition to be updated in cache
			err = wait.PollUntilContextTimeout(ctx, 500*time.Millisecond, 10*time.Second, false, func(ctx context.Context) (done bool, err error) {
				updatedNm := &maintenancev1.NodeMaintenance{}
				innerErr := r.Client.Get(ctx, types.NamespacedName{Namespace: nm.Namespace, Name: nm.Name}, updatedNm)
				if innerErr != nil {
					if k8serrors.IsNotFound(innerErr) {
						return true, nil
					}
					r.Log.Error(innerErr, "failed to get NodeMaintenance object while waiting for condition update. retrying", "name", nm.Name, "namespace", nm.Namespace)
					return false, nil
				}
				if k8sutils.GetReadyConditionReason(updatedNm) == maintenancev1.ConditionReasonScheduled {
					return true, nil
				}
				return false, nil
			})
			if err != nil {
				// Note(adrianc): if this happens we rely on the fact that caches are updated until next reconcile call
				r.Log.Error(err, "failed while waiting for condition for NodeMaintenance", "name", nm.Name, "namespace", nm.Namespace)
			}
		}()
	}
	// wait for all updates to finish
	wg.Wait()

	return ctrl.Result{RequeueAfter: schedulerSyncTime}, nil
}

// preSchedule performs common pre-scheduling checks if we can schedule new NodeMaintenance requests. returns preScheduleContext or error.
func (r *NodeMaintenanceSchedulerReconciler) preSchedule(nodes []*corev1.Node, nodeMaintenances []*maintenancev1.NodeMaintenance) (*scheduler.SchedulerContext, error) {
	nodesAsSet := sets.New[string]()
	for _, n := range nodes {
		nodesAsSet.Insert(n.Name)
	}

	// nodesUnderMaintenance are nodes that have NodeMaintenance request in progress and their respective node exists
	nodesUnderMaintenance := sets.New[string]()
	for _, nm := range nodeMaintenances {
		if k8sutils.IsUnderMaintenance(nm) && nodesAsSet.Has(nm.Spec.NodeName) {
			nodesUnderMaintenance.Insert(nm.Spec.NodeName)
		}
	}

	r.Log.V(operatorlog.DebugLevel).Info("nodesUnderMaintenance", "value", nodesUnderMaintenance.UnsortedList())

	// nodesPendingMaintenance are nodes that have one(or more) NodeMaintenance request in Pending state and their respective node exists
	nodesPendingMaintenance := sets.New[string]()
	for _, nm := range nodeMaintenances {
		if k8sutils.GetReadyConditionReason(nm) == maintenancev1.ConditionReasonPending &&
			nodesAsSet.Has(nm.Spec.NodeName) {
			nodesPendingMaintenance.Insert(nm.Spec.NodeName)
		}
	}

	r.Log.V(operatorlog.DebugLevel).Info("nodesPendingMaintenance", "value", nodesPendingMaintenance.UnsortedList())

	// nodesUnavailable are the currently unavailable nodes in the cluster
	nodesUnavailable := sets.New(r.getCurrentUnavailableNodes(nodes)...)
	r.Log.V(operatorlog.DebugLevel).Info("nodesUnavailable", "value", nodesUnavailable.UnsortedList())

	// totalNodeUnavailable is the union of nodesUnavailable and nodesUnderMaintenance i.e nodes that are and will be (at some point)
	// unavailable.
	totalNodeUnavailable := nodesUnavailable.Union(nodesUnderMaintenance)
	r.Log.V(operatorlog.DebugLevel).Info("totalNodeUnavailable", "total", totalNodeUnavailable.UnsortedList())

	maxUnavailable, err := r.getMaxUnavailableNodes(len(nodes))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to calculate max unavailable nodes allowed in the cluster")
	}

	maxOperations, err := r.getMaxOperations(len(nodes))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to calculate max parallel operations allowed in the cluster")
	}

	availableSlots := utils.Min(utils.Max(0, maxOperations-len(nodesUnderMaintenance)), len(nodes))

	canBecomeUnavailable := availableSlots
	if maxUnavailable >= 0 {
		canBecomeUnavailable = utils.Max(0, maxUnavailable-len(totalNodeUnavailable))
	}

	// we cannot schedule New NodeMaintenance on nodes that currently undergo maintenance candidateNodes holds nodes that have pending
	// NodeMaintenance request which are not currently undergoing maintenance
	candidateNodes := nodesPendingMaintenance.Difference(nodesUnderMaintenance)

	var candidateMaintenance []*maintenancev1.NodeMaintenance
	if len(candidateNodes) > 0 {
		for i := range nodeMaintenances {
			if candidateNodes.Has(nodeMaintenances[i].Spec.NodeName) {
				candidateMaintenance = append(candidateMaintenance, nodeMaintenances[i])
			}
		}
	}

	r.Log.Info("stats:",
		"maxUnavailable", maxUnavailable,
		"maxOperations", maxOperations,
		"nodesPendingMaintenance", len(nodesPendingMaintenance),
		"candidateNodes", len(candidateNodes),
		"availableSlots", availableSlots,
		"canBecomeUnavailable", canBecomeUnavailable,
		"candidateMaintenance", len(candidateMaintenance),
	)

	r.Log.V(operatorlog.DebugLevel).Info("candidateNodes", "value", candidateNodes.UnsortedList())
	if r.Log.GetV() == operatorlog.DebugLevel {
		candidateMaintenanceNames := make([]string, 0, len(candidateMaintenance))
		for _, nm := range candidateMaintenance {
			candidateMaintenanceNames = append(candidateMaintenanceNames, nm.CanonicalString())
		}
		r.Log.V(operatorlog.DebugLevel).Info("candidateMaintenance", "value", candidateMaintenanceNames)
	}

	return &scheduler.SchedulerContext{
		AvailableSlots:       availableSlots,
		CanBecomeUnavailable: canBecomeUnavailable,
		CandidateNodes:       candidateNodes,
		CandidateMaintenance: candidateMaintenance,
	}, nil
}

// getMaxUnavailableNodes returns the absolute number of unavailable nodes allowed or -1 if no limit.
func (r *NodeMaintenanceSchedulerReconciler) getMaxUnavailableNodes(totalNumOfNodes int) (int, error) {
	maxUnavailable := r.Options.MaxUnavailable()
	// unset means unlimited
	if maxUnavailable == nil {
		return -1, nil
	}
	maxUnavail, err := intstr.GetScaledValueFromIntOrPercent(maxUnavailable, totalNumOfNodes, false)
	if err != nil {
		return -1, err
	}

	return maxUnavail, nil
}

// getMaxOperations returns the absolute number of operations allowed.
func (r *NodeMaintenanceSchedulerReconciler) getMaxOperations(totalNumOfNodes int) (int, error) {
	maxParallelOperations := r.Options.MaxParallelOperations()
	// unset, use defaut of 1
	if maxParallelOperations == nil {
		return 1, nil
	}
	maxOper, err := intstr.GetScaledValueFromIntOrPercent(maxParallelOperations, totalNumOfNodes, true)
	if err != nil {
		return -1, err
	}

	return maxOper, nil
}

// getCurrentUnavailableNodes returns the current unavailable node names
func (r *NodeMaintenanceSchedulerReconciler) getCurrentUnavailableNodes(nl []*corev1.Node) []string {
	var unavailableNodes []string
	for _, node := range nl {
		// check if the node is cordoned or is not ready
		if k8sutils.IsNodeUnschedulable(node) || !k8sutils.IsNodeReady(node) {
			unavailableNodes = append(unavailableNodes, node.Name)
		}
	}
	return unavailableNodes
}

// SetupWithManager sets up the controller with the Manager.
func (r *NodeMaintenanceSchedulerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	qHandler := func(q workqueue.RateLimitingInterface) {
		q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
			Namespace: "",
			Name:      schedulerSyncEventName,
		}})
	}

	eventHandler := handler.Funcs{
		GenericFunc: func(ctx context.Context, e event.GenericEvent, q workqueue.RateLimitingInterface) {
			log.Log.WithName("NodeMaintenanceScheduler").
				Info("Enqueuing sync for generic event", "resource", e.Object.GetName())
			qHandler(q)
		},
	}

	// send initial sync event to trigger reconcile when controller is started
	eventChan := make(chan event.GenericEvent, 1)
	eventChan <- event.GenericEvent{Object: &maintenancev1.NodeMaintenance{
		ObjectMeta: metav1.ObjectMeta{Name: schedulerSyncEventName, Namespace: ""},
	}}
	close(eventChan)

	// setup event recorder
	r.EventRecorder = mgr.GetEventRecorderFor("nodemaintenancescheduler")

	return ctrl.NewControllerManagedBy(mgr).
		Named("nodemaintenancescheduler").
		WatchesRawSource(source.Channel(eventChan, eventHandler)).
		Complete(r)
}
