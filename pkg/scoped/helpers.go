package scoped

import (
	"context"
	"fmt"

	authv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
)

func createSSAR(cli dynamic.Interface, ssar *authv1.SelfSubjectAccessReview) (*authv1.SelfSubjectAccessReview, error) {
	ssarUC, err := runtime.DefaultUnstructuredConverter.ToUnstructured(ssar)
	if err != nil {
		return nil, fmt.Errorf("encountered an error converting to unstructured: %w", err)
	}

	uSSAR := &unstructured.Unstructured{}
	uSSAR.SetGroupVersionKind(ssar.GroupVersionKind())
	uSSAR.Object = ssarUC

	ssarClient := cli.Resource(authv1.SchemeGroupVersion.WithResource("selfsubjectaccessreviews"))
	uCreatedSSAR, err := ssarClient.Create(context.TODO(), uSSAR, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("encountered an error creating a cluster level SSAR: %w", err)
	}

	createdSSAR := &authv1.SelfSubjectAccessReview{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(uCreatedSSAR.UnstructuredContent(), createdSSAR)
	if err != nil {
		return nil, fmt.Errorf("encountered an error converting from unstructured: %w", err)
	}

	return createdSSAR, nil
}

func canClusterVerbResource(cli dynamic.Interface, gvr schema.GroupVersionResource, verb string) (bool, error) {
	// Check if we have cluster permissions to list the resource
	// create the cluster level SelfSubjectAccessReview
	cSSAR := &authv1.SelfSubjectAccessReview{
		Spec: authv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authv1.ResourceAttributes{
				Verb:     verb,
				Group:    gvr.Group,
				Version:  gvr.Version,
				Resource: gvr.Resource,
			},
		},
	}

	createdClusterSSAR, err := createSSAR(cli, cSSAR)
	if err != nil {
		return false, fmt.Errorf("encountered an error creating a cluster level SSAR: %w", err)
	}

	return createdClusterSSAR.Status.Allowed, nil
}

func getNamespacesForVerbResource(cli dynamic.Interface, gvr schema.GroupVersionResource, verb string) ([]corev1.Namespace, error) {
	permittedNs := []corev1.Namespace{}
	nsClient := cli.Resource(corev1.SchemeGroupVersion.WithResource("namespaces"))
	nsList := &corev1.NamespaceList{}
	uNsList, err := nsClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("encountered an error when getting the list of namespaces on the cluster: %w", err)
	}

	err = runtime.DefaultUnstructuredConverter.FromUnstructured(uNsList.UnstructuredContent(), nsList)
	if err != nil {
		return nil, fmt.Errorf("encountered an error converting from unstructured: %w", err)
	}

	for _, ns := range nsList.Items {
		nsSSAR := &authv1.SelfSubjectAccessReview{
			Spec: authv1.SelfSubjectAccessReviewSpec{
				ResourceAttributes: &authv1.ResourceAttributes{
					Namespace: ns.Name,
					Verb:      verb,
					Group:     gvr.Group,
					Version:   gvr.Version,
					Resource:  gvr.Resource,
				},
			},
		}

		createdNsSSAR, err := createSSAR(cli, nsSSAR)
		if err != nil {
			return nil, fmt.Errorf("encountered an error creating a namespace level SSAR: %w", err)
		}

		if createdNsSSAR.Status.Allowed {
			permittedNs = append(permittedNs, ns)
		}
	}

	return permittedNs, nil
}

func listResourcesForNamespaces(cli dynamic.Interface, gvr schema.GroupVersionResource, namespaces []corev1.Namespace, options metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	uList := &unstructured.UnstructuredList{}

	for _, ns := range namespaces {
		temp, err := cli.Resource(gvr).Namespace(ns.Name).List(context.TODO(), options)
		if err != nil {
			return nil, fmt.Errorf("encountered an error getting a list of resources from namespaces: %w", err)
		}

		uList.Items = append(uList.Items, temp.Items...)
		uList.SetResourceVersion(temp.GetResourceVersion())
	}

	return uList, nil
}

func watchResourcesForNamespaces(cli dynamic.Interface, gvr schema.GroupVersionResource, namespaces []corev1.Namespace, options metav1.ListOptions) (watch.Interface, error) {
	watchChannels := []<-chan watch.Event{}

	for _, ns := range namespaces {
		w, err := cli.Resource(gvr).Namespace(ns.Name).Watch(context.TODO(), options)
		if err != nil {
			return nil, fmt.Errorf("encountered an error getting a watch of resources from namespaces: %w", err)
		}

		watchChannels = append(watchChannels, w.ResultChan())
	}

	return NewScopedWatcher(watchChannels...), nil
}
