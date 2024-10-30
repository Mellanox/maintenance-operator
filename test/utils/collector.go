/*
Copyright 2024 NVIDIA

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
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	maintenancev1 "github.com/Mellanox/maintenance-operator/api/v1alpha1"
)

// ClusterCollector collects resources from the cluster and stores them in artifacts directory.
type ClusterCollector struct {
	client       client.Client
	clientset    *kubernetes.Clientset
	artifactsDir string
}

func NewClusterCollector(
	client client.Client, clientset *kubernetes.Clientset, artifactsDirectory string,
) *ClusterCollector {
	return &ClusterCollector{
		client:       client,
		clientset:    clientset,
		artifactsDir: artifactsDirectory,
	}
}

func (c *ClusterCollector) Run(ctx context.Context) error {
	// You can add entries here for resources that are not part of the inventory. Inventory resources are collected
	// automatically.
	resourcesToCollect := []schema.GroupVersionKind{
		corev1.SchemeGroupVersion.WithKind("Pod"),                        // Pods
		corev1.SchemeGroupVersion.WithKind("Node"),                       // Nodes
		appsv1.SchemeGroupVersion.WithKind("Deployment"),                 // Deployments
		maintenancev1.GroupVersion.WithKind("NodeMaintenance"),           // NodeMaintenances
		maintenancev1.GroupVersion.WithKind("MaintenanceOperatorConfig"), // MaintenanceOperatorConfig
	}
	errs := make([]error, 0)

	for _, resource := range resourcesToCollect {
		gvkExists, err := verifyGVKExists(c.client, resource)
		if err != nil {
			errs = append(errs, fmt.Errorf("error verifying GVK: %v", err))
		}
		if !gvkExists {
			continue
		}
		err = c.dumpResource(ctx, resource)
		if err != nil {
			errs = append(errs, fmt.Errorf("error dumping %vs %w", resource.Kind, err))
		}
	}

	// Dump the logs from all the pods on the cluster.+
	err := c.dumpPodLogsAndEvents(ctx)
	if err != nil {
		errs = append(errs, fmt.Errorf("error dumping pod logs %w", err))
	}

	nsList := &corev1.NamespaceList{}
	if err := c.client.List(ctx, nsList); err == nil {
		for _, ns := range nsList.Items {
			if err := c.dumpEventsForNamespace(ctx, ns.Name); err != nil {
				errs = append(errs, fmt.Errorf("error dumping events for namespace %s: %w", ns.Name, err))
			}
		}
	} else {
		errs = append(errs, fmt.Errorf("error listing namespaces will not dump events: %w", err))
	}

	return kerrors.NewAggregate(errs)
}

func verifyGVKExists(c client.Client, gvk schema.GroupVersionKind) (bool, error) {
	mapper := c.RESTMapper()

	// Try to map the GVK to a resource.
	_, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		if meta.IsNoMatchError(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func (c *ClusterCollector) dumpPodLogsAndEvents(ctx context.Context) error {
	podList := &corev1.PodList{}
	err := c.client.List(ctx, podList)
	if err != nil {
		return err
	}
	errs := []error{}
	for _, pod := range podList.Items {
		if err = c.dumpLogsForPod(ctx, &pod); err != nil {
			errs = append(errs, err)
		}
		if err = c.dumpEventsForNamespacedResource(ctx, "Pod", client.ObjectKeyFromObject(&pod)); err != nil {
			errs = append(errs, err)
		}
	}
	return kerrors.NewAggregate(errs)
}

func (c *ClusterCollector) dumpLogsForPod(ctx context.Context, pod *corev1.Pod) (reterr error) {
	errs := []error{}
	for _, container := range pod.Spec.Containers {
		podLogOpts := corev1.PodLogOptions{Container: container.Name}
		if err := c.dumpLogsForContainer(ctx, pod.Namespace, pod.Name, "", podLogOpts); err != nil {
			errs = append(errs, err)
		}

		// Also collect the logs from a previous container if one existed.
		previousContainerOpts := corev1.PodLogOptions{Container: container.Name, Previous: true}
		if err := c.dumpLogsForContainer(ctx, pod.Namespace, pod.Name, ".previous", previousContainerOpts); err != nil {
			if !strings.Contains(err.Error(), "not found") {
				errs = append(errs, err)
			}
		}
	}
	return kerrors.NewAggregate(errs)
}

func (c *ClusterCollector) dumpLogsForContainer(ctx context.Context, podNamespace, podName,
	fileSuffix string, options corev1.PodLogOptions,
) (reterr error) {
	req := c.clientset.CoreV1().Pods(podNamespace).GetLogs(podName, &options)
	podLogs, err := req.Stream(ctx)
	if err != nil {
		return err
	}
	defer func() {
		err := podLogs.Close()
		if err != nil {
			reterr = err
		}
	}()

	logs := new(bytes.Buffer)
	_, err = io.Copy(logs, podLogs)
	if err != nil {
		return err
	}
	filePath := filepath.Join(
		c.artifactsDir, "Logs", podNamespace, podName, fmt.Sprintf("%v.log%s", options.Container, fileSuffix))
	if err := c.writeToFile(logs.Bytes(), filePath); err != nil {
		return err
	}
	return nil
}

func (c *ClusterCollector) writeToFile(data []byte, filePath string) error {
	err := os.MkdirAll(filepath.Dir(filePath), 0o750)
	if err != nil {
		return err
	}
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	err = os.WriteFile(f.Name(), data, 0o600)
	if err != nil {
		return err
	}
	return nil
}

func (c *ClusterCollector) dumpEventsForNamespacedResource(
	ctx context.Context, kind string, ref types.NamespacedName,
) error {
	fieldSelector := fmt.Sprintf("regarding.name=%s", ref.Name)
	events, _ := c.clientset.EventsV1().Events(ref.Namespace).List(
		ctx, metav1.ListOptions{FieldSelector: fieldSelector, TypeMeta: metav1.TypeMeta{Kind: kind}})
	filePath := filepath.Join(c.artifactsDir, "Events", kind, ref.Namespace, fmt.Sprintf("%v.events", ref.Name))
	if err := c.writeResourceToFile(events, filePath); err != nil {
		return err
	}
	return nil
}

func (c *ClusterCollector) dumpEventsForNamespace(ctx context.Context, namespace string) error {
	events, err := c.clientset.EventsV1().Events(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error while listing events: %w", err)
	}
	filePath := filepath.Join(c.artifactsDir, "Events", "Namespace", fmt.Sprintf("%v.events", namespace))
	if err := c.writeResourceToFile(events, filePath); err != nil {
		return err
	}
	return nil
}

func (c *ClusterCollector) dumpResource(ctx context.Context, kind schema.GroupVersionKind) error {
	resourceList := unstructured.UnstructuredList{}
	resourceList.SetKind(kind.Kind)
	resourceList.SetAPIVersion(kind.GroupVersion().String())
	if err := c.client.List(ctx, &resourceList); err != nil {
		return err
	}
	for _, resource := range resourceList.Items {
		filePath := filepath.Join(
			c.artifactsDir, "Resources", resource.GetObjectKind().GroupVersionKind().Kind, resource.GetNamespace(),
			fmt.Sprintf("%v.yaml", resource.GetName()))
		err := c.writeResourceToFile(&resource, filePath)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *ClusterCollector) writeResourceToFile(resource runtime.Object, filePath string) error {
	yaml, err := yaml.Marshal(resource)
	if err != nil {
		return err
	}
	if err := c.writeToFile(yaml, filePath); err != nil {
		return err
	}
	return nil
}
