package handler

import (
	"context"
	"fmt"
	"strings"

	"github.com/everettraven/rbac-proxy-poc/internal/rbac"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// getResourceForKind is a helper function for getting the pluralized resource for a given Kind
func getResourceForKind(kind string) string {
	// lowercase and pluralized Kind represents the resource
	return strings.ToLower(kind) + "s"
}

// getPermittedNamespaces is a helper function to get a list of namespaces that have the given verb as
// a permission for the provided GroupVersionKind. It returns a list of namespaces.
func getPermittedNamespaces(nsPerms rbac.NamespacedPermissions, gvk schema.GroupVersionKind, verb string) []string {
	namespaces := []string{}
	resource := getResourceForKind(gvk.Kind)
	for namespace, permissions := range nsPerms {
		if _, ok := permissions[resource]; ok {
			if _, ok := permissions[resource]["*"]; ok { // has all permissions for the resource
				namespaces = append(namespaces, namespace)
			} else if _, ok := permissions[resource][verb]; ok { // has verb permissions for the resource
				namespaces = append(namespaces, namespace)
			}
		}
	}

	return namespaces
}

// getKindList is a helper function to get the list Kind of a given Kind (i.e. PodList from Pod)
func getKindList(kind string) string {
	return kind + "List"
}

// getNamespacedResourceList is a helper function that when given a
// client.Client, GroupVersionKind, NamespacePermissions, and a verb it will return a list
// of resources from all the namespaces that include the permission verb provided for the given
// GVK condensed into one resource list. It returns an unstructured.UnstructuredList.
func getNamespacedResourceList(cli client.Client, gvk schema.GroupVersionKind, nsPerms rbac.NamespacedPermissions, verb string) *unstructured.UnstructuredList {
	namespaces := getPermittedNamespaces(nsPerms, gvk, verb)

	// Create the resource list GVK
	listGVK := schema.GroupVersionKind{
		Group:   gvk.Group,
		Version: gvk.Version,
		Kind:    getKindList(gvk.Kind),
	}

	resourceList := &unstructured.UnstructuredList{}
	resourceList.SetGroupVersionKind(listGVK)

	for _, ns := range namespaces {
		tempList := &unstructured.UnstructuredList{}
		tempList.SetGroupVersionKind(listGVK)

		err := cli.List(context.Background(), tempList, &client.ListOptions{
			Namespace: ns,
		})
		if err != nil {
			klog.V(0).ErrorS(err, fmt.Sprintf("encountered an error getting %s for namespace `%s`", tempList.GetKind(), ns))
			continue
		}

		// append items to list
		resourceList.Items = append(resourceList.Items, tempList.Items...)

		// set the resourceVersion in the event it needs to be used in a watches request
		resourceList.SetResourceVersion(tempList.GetResourceVersion())
	}

	return resourceList
}
