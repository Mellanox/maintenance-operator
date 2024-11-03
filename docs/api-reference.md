Packages:

-   [maintenance.nvidia.com/v1alpha1](#maintenancenvidiacomv1alpha1)

## maintenance.nvidia.com/v1alpha1

Package v1alpha1 contains API Schema definitions for the maintenance.nvidia.com v1alpha1 API group

Resource Types:

### DrainSpec

(*Appears on:*[NodeMaintenanceSpec](#NodeMaintenanceSpec))

DrainSpec describes configuration for node drain during automatic upgrade

<table>
<colgroup>
<col style="width: 50%" />
<col style="width: 50%" />
</colgroup>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>force</code><br />
<em>bool</em></td>
<td><p>Force draining even if there are pods that do not declare a controller</p></td>
</tr>
<tr>
<td><code>podSelector</code><br />
<em>string</em></td>
<td><p>PodSelector specifies a label selector to filter pods on the node that need to be drained For more details on label selectors, see: <a
href="https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors">https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors</a></p></td>
</tr>
<tr>
<td><code>timeoutSeconds</code><br />
<em>int32</em></td>
<td><p>TimeoutSecond specifies the length of time in seconds to wait before giving up drain, zero means infinite</p></td>
</tr>
<tr>
<td><code>deleteEmptyDir</code><br />
<em>bool</em></td>
<td><p>DeleteEmptyDir indicates if should continue even if there are pods using emptyDir (local data that will be deleted when the node is drained)</p></td>
</tr>
<tr>
<td><code>podEvictionFilters</code><br />
<em><a href="#PodEvictionFiterEntry">[]PodEvictionFiterEntry</a></em></td>
<td><p>PodEvictionFilters specifies filters for pods that need to undergo eviction during drain. if specified. only pods that match PodEvictionFilters will be evicted during drain operation. if
unspecified. all non-daemonset pods will be evicted. logical OR is performed between filter entires. logical AND is performed within different filters in a filter entry.</p></td>
</tr>
</tbody>
</table>

### DrainStatus

(*Appears on:*[NodeMaintenanceStatus](#NodeMaintenanceStatus))

DrainStatus represents the status of draining for the node

<table>
<colgroup>
<col style="width: 50%" />
<col style="width: 50%" />
</colgroup>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>totalPods</code><br />
<em>int32</em></td>
<td><p>TotalPods is the number of pods on the node at the time NodeMaintenance started draining</p></td>
</tr>
<tr>
<td><code>evictionPods</code><br />
<em>int32</em></td>
<td><p>EvictionPods is the total number of pods that need to be evicted at the time NodeMaintenance started draining</p></td>
</tr>
<tr>
<td><code>drainProgress</code><br />
<em>int32</em></td>
<td><p>DrainProgress represents the draining progress as percentage</p></td>
</tr>
<tr>
<td><code>waitForEviction</code><br />
<em>[]string</em></td>
<td><p>WaitForEviction is the list of namespaced named pods that need to be evicted</p></td>
</tr>
</tbody>
</table>

### MaintenanceOperatorConfig

MaintenanceOperatorConfig is the Schema for the maintenanceoperatorconfigs API

<table>
<colgroup>
<col style="width: 50%" />
<col style="width: 50%" />
</colgroup>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>metadata</code><br />
<em><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#objectmeta-v1-meta">Kubernetes meta/v1.ObjectMeta</a></em></td>
<td>Refer to the Kubernetes API documentation for the fields of the <code>metadata</code> field.</td>
</tr>
<tr>
<td><code>spec</code><br />
<em><a href="#MaintenanceOperatorConfigSpec">MaintenanceOperatorConfigSpec</a></em></td>
<td><br />
<br />
&#10;<table>
<colgroup>
<col style="width: 50%" />
<col style="width: 50%" />
</colgroup>
<tbody>
<tr>
<td><code>maxParallelOperations</code><br />
<em><a href="https://pkg.go.dev/k8s.io/apimachinery/pkg/util/intstr#IntOrString">k8s.io/apimachinery/pkg/util/intstr.IntOrString</a></em></td>
<td><p>MaxParallelOperations indicates the maximal number nodes that can undergo maintenance at a given time. 0 means no limit value can be an absolute number (ex: 5) or a percentage of total nodes in
the cluster (ex: 10%). absolute number is calculated from percentage by rounding up. defaults to 1. The actual number of nodes that can undergo maintenance may be lower depending on the value of
MaintenanceOperatorConfigSpec.MaxUnavailable.</p></td>
</tr>
<tr>
<td><code>maxUnavailable</code><br />
<em><a href="https://pkg.go.dev/k8s.io/apimachinery/pkg/util/intstr#IntOrString">k8s.io/apimachinery/pkg/util/intstr.IntOrString</a></em></td>
<td><p>MaxUnavailable is the maximum number of nodes that can become unavailable in the cluster. value can be an absolute number (ex: 5) or a percentage of total nodes in the cluster (ex: 10%).
absolute number is calculated from percentage by rounding up. by default, unset. new nodes will not be processed if the number of unavailable node will exceed this value</p></td>
</tr>
<tr>
<td><code>logLevel</code><br />
<em><a href="#OperatorLogLevel-string-alias">OperatorLogLevel</a></em></td>
<td><p>LogLevel is the operator logging level</p></td>
</tr>
<tr>
<td><code>maxNodeMaintenanceTimeSeconds</code><br />
<em>int32</em></td>
<td><p>MaxNodeMaintenanceTimeSeconds is the time from when a NodeMaintenance is marked as ready (phase: Ready) until the NodeMaintenance is considered stale and removed by the operator. should be less
than idle time for any autoscaler that is running. default to 30m (1600 seconds)</p></td>
</tr>
</tbody>
</table></td>
</tr>
</tbody>
</table>

### MaintenanceOperatorConfigSpec

(*Appears on:*[MaintenanceOperatorConfig](#MaintenanceOperatorConfig))

MaintenanceOperatorConfigSpec defines the desired state of MaintenanceOperatorConfig

<table>
<colgroup>
<col style="width: 50%" />
<col style="width: 50%" />
</colgroup>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>maxParallelOperations</code><br />
<em><a href="https://pkg.go.dev/k8s.io/apimachinery/pkg/util/intstr#IntOrString">k8s.io/apimachinery/pkg/util/intstr.IntOrString</a></em></td>
<td><p>MaxParallelOperations indicates the maximal number nodes that can undergo maintenance at a given time. 0 means no limit value can be an absolute number (ex: 5) or a percentage of total nodes in
the cluster (ex: 10%). absolute number is calculated from percentage by rounding up. defaults to 1. The actual number of nodes that can undergo maintenance may be lower depending on the value of
MaintenanceOperatorConfigSpec.MaxUnavailable.</p></td>
</tr>
<tr>
<td><code>maxUnavailable</code><br />
<em><a href="https://pkg.go.dev/k8s.io/apimachinery/pkg/util/intstr#IntOrString">k8s.io/apimachinery/pkg/util/intstr.IntOrString</a></em></td>
<td><p>MaxUnavailable is the maximum number of nodes that can become unavailable in the cluster. value can be an absolute number (ex: 5) or a percentage of total nodes in the cluster (ex: 10%).
absolute number is calculated from percentage by rounding up. by default, unset. new nodes will not be processed if the number of unavailable node will exceed this value</p></td>
</tr>
<tr>
<td><code>logLevel</code><br />
<em><a href="#OperatorLogLevel-string-alias">OperatorLogLevel</a></em></td>
<td><p>LogLevel is the operator logging level</p></td>
</tr>
<tr>
<td><code>maxNodeMaintenanceTimeSeconds</code><br />
<em>int32</em></td>
<td><p>MaxNodeMaintenanceTimeSeconds is the time from when a NodeMaintenance is marked as ready (phase: Ready) until the NodeMaintenance is considered stale and removed by the operator. should be less
than idle time for any autoscaler that is running. default to 30m (1600 seconds)</p></td>
</tr>
</tbody>
</table>

### NodeMaintenance

NodeMaintenance is the Schema for the nodemaintenances API

<table>
<colgroup>
<col style="width: 50%" />
<col style="width: 50%" />
</colgroup>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>metadata</code><br />
<em><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#objectmeta-v1-meta">Kubernetes meta/v1.ObjectMeta</a></em></td>
<td>Refer to the Kubernetes API documentation for the fields of the <code>metadata</code> field.</td>
</tr>
<tr>
<td><code>spec</code><br />
<em><a href="#NodeMaintenanceSpec">NodeMaintenanceSpec</a></em></td>
<td><br />
<br />
&#10;<table>
<colgroup>
<col style="width: 50%" />
<col style="width: 50%" />
</colgroup>
<tbody>
<tr>
<td><code>requestorID</code><br />
<em>string</em></td>
<td><p>RequestorID MUST follow domain name notation format (<a href="https://tools.ietf.org/html/rfc1035#section-2.3.1">https://tools.ietf.org/html/rfc1035#section-2.3.1</a>) It MUST be 63 characters
or less, beginning and ending with an alphanumeric character ([a-z0-9A-Z]) with dashes (-), dots (.), and alphanumerics between. caller SHOULD NOT create multiple objects with same requestorID and
nodeName. This field identifies the requestor of the operation.</p></td>
</tr>
<tr>
<td><code>additionalRequestors</code><br />
<em>[]string</em></td>
<td><p>AdditionalRequestors is a set of additional requestor IDs which are using the same NodeMaintenance request. addition or removal of requiestor IDs to this list MUST be made with update operation
(and retry on failure) which will replace the entire list.</p></td>
</tr>
<tr>
<td><code>nodeName</code><br />
<em>string</em></td>
<td><p>NodeName is The name of the node that maintenance operation will be performed on creation fails if node obj does not exist (webhook)</p></td>
</tr>
<tr>
<td><code>cordon</code><br />
<em>bool</em></td>
<td><p>Cordon if set, marks node as unschedulable during maintenance operation</p></td>
</tr>
<tr>
<td><code>waitForPodCompletion</code><br />
<em><a href="#WaitForPodCompletionSpec">WaitForPodCompletionSpec</a></em></td>
<td><p>WaitForPodCompletion specifies pods via selector to wait for completion before performing drain operation if not provided, will not wait for pods to complete</p></td>
</tr>
<tr>
<td><code>drainSpec</code><br />
<em><a href="#DrainSpec">DrainSpec</a></em></td>
<td><p>DrainSpec specifies how a node will be drained. if not provided, no draining will be performed.</p></td>
</tr>
</tbody>
</table></td>
</tr>
<tr>
<td><code>status</code><br />
<em><a href="#NodeMaintenanceStatus">NodeMaintenanceStatus</a></em></td>
<td></td>
</tr>
</tbody>
</table>

### NodeMaintenanceSpec

(*Appears on:*[NodeMaintenance](#NodeMaintenance))

NodeMaintenanceSpec defines the desired state of NodeMaintenance

<table>
<colgroup>
<col style="width: 50%" />
<col style="width: 50%" />
</colgroup>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>requestorID</code><br />
<em>string</em></td>
<td><p>RequestorID MUST follow domain name notation format (<a href="https://tools.ietf.org/html/rfc1035#section-2.3.1">https://tools.ietf.org/html/rfc1035#section-2.3.1</a>) It MUST be 63 characters
or less, beginning and ending with an alphanumeric character ([a-z0-9A-Z]) with dashes (-), dots (.), and alphanumerics between. caller SHOULD NOT create multiple objects with same requestorID and
nodeName. This field identifies the requestor of the operation.</p></td>
</tr>
<tr>
<td><code>additionalRequestors</code><br />
<em>[]string</em></td>
<td><p>AdditionalRequestors is a set of additional requestor IDs which are using the same NodeMaintenance request. addition or removal of requiestor IDs to this list MUST be made with update operation
(and retry on failure) which will replace the entire list.</p></td>
</tr>
<tr>
<td><code>nodeName</code><br />
<em>string</em></td>
<td><p>NodeName is The name of the node that maintenance operation will be performed on creation fails if node obj does not exist (webhook)</p></td>
</tr>
<tr>
<td><code>cordon</code><br />
<em>bool</em></td>
<td><p>Cordon if set, marks node as unschedulable during maintenance operation</p></td>
</tr>
<tr>
<td><code>waitForPodCompletion</code><br />
<em><a href="#WaitForPodCompletionSpec">WaitForPodCompletionSpec</a></em></td>
<td><p>WaitForPodCompletion specifies pods via selector to wait for completion before performing drain operation if not provided, will not wait for pods to complete</p></td>
</tr>
<tr>
<td><code>drainSpec</code><br />
<em><a href="#DrainSpec">DrainSpec</a></em></td>
<td><p>DrainSpec specifies how a node will be drained. if not provided, no draining will be performed.</p></td>
</tr>
</tbody>
</table>

### NodeMaintenanceStatus

(*Appears on:*[NodeMaintenance](#NodeMaintenance))

NodeMaintenanceStatus defines the observed state of NodeMaintenance

<table>
<colgroup>
<col style="width: 50%" />
<col style="width: 50%" />
</colgroup>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>conditions</code><br />
<em><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#condition-v1-meta">[]Kubernetes meta/v1.Condition</a></em></td>
<td><p>Conditions represents observations of NodeMaintenance current state</p></td>
</tr>
<tr>
<td><code>waitForCompletion</code><br />
<em>[]string</em></td>
<td><p>WaitForCompletion is the list of namespaced named pods that we wait to complete</p></td>
</tr>
<tr>
<td><code>drain</code><br />
<em><a href="#DrainStatus">DrainStatus</a></em></td>
<td><p>Drain represents the drain status of the node</p></td>
</tr>
</tbody>
</table>

### OperatorLogLevel (`string` alias)

(*Appears on:*[MaintenanceOperatorConfigSpec](#MaintenanceOperatorConfigSpec))

OperatorLogLevel is the operator log level. one of: \[“debug”, “info”, “error”\]

### PodEvictionFiterEntry

(*Appears on:*[DrainSpec](#DrainSpec))

PodEvictionFiterEntry defines filters for Pod evictions during drain operation

<table>
<colgroup>
<col style="width: 50%" />
<col style="width: 50%" />
</colgroup>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>byResourceNameRegex</code><br />
<em>string</em></td>
<td><p>ByResourceNameRegex filters pods by the name of the resources they consume using regex.</p></td>
</tr>
</tbody>
</table>

### WaitForPodCompletionSpec

(*Appears on:*[NodeMaintenanceSpec](#NodeMaintenanceSpec))

WaitForPodCompletionSpec describes the configuration for waiting on pods completion

<table>
<colgroup>
<col style="width: 50%" />
<col style="width: 50%" />
</colgroup>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td><code>podSelector</code><br />
<em>string</em></td>
<td><p>PodSelector specifies a label selector for the pods to wait for completion For more details on label selectors, see: <a
href="https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors">https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors</a></p></td>
</tr>
<tr>
<td><code>timeoutSeconds</code><br />
<em>int32</em></td>
<td><p>TimeoutSecond specifies the length of time in seconds to wait before giving up on pod termination, zero means infinite</p></td>
</tr>
</tbody>
</table>

--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------

*Generated with `gen-crd-api-reference-docs` on git commit `fb44536`.*
