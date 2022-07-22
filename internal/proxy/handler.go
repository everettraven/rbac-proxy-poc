package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/everettraven/rbac-proxy-poc/internal/rbac"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

//, rbac *rbac.RBACWatcher

func HandleRequest(rw http.ResponseWriter, req *http.Request, rbac *rbac.RBACWatcher) bool {
	direct := false

	// create a client
	cli, err := client.New(config.GetConfigOrDie(), client.Options{})
	if err != nil {
		klog.V(0).ErrorS(err, "encountered an error creating a new controller-runtime client")
		return direct
	}

	// For now just test the ability to stream responses when we recieve a watch request
	// TODO: implement this for real
	if isWatchRequest(req.URL) {
		for i := 0; i < 5; i++ {
			rw.Write([]byte(fmt.Sprintf("Hello #%d\n", i)))
			if f, ok := rw.(http.Flusher); ok {
				f.Flush()
			}
			time.Sleep(1 * time.Second)
		}

		return direct
	}

	if isSpecificRequest(req.URL) { // if a specific request proxy directly to the kube api
		direct = true
	} else {
		if isClusterScopedRequest(req.URL) {
			if isListRequest(req.URL) {
				gvk := gvkFromURL(req.URL)
				// lowercase and pluralized Kind represents the resource
				resource := strings.ToLower(gvk.Kind) + "s"
				if _, ok := rbac.ClusterPermissions[resource]; ok { // has some form of cluster permissions for the resource
					if _, ok := rbac.ClusterPermissions[resource]["*"]; ok { // has all permissions for the resource
						direct = true
					} else if _, ok := rbac.ClusterPermissions[resource]["list"]; ok { // has cluster list permissions for the resource
						direct = true
					} else { // time to fake the cluster request
						resourceList := getNamespacedResourceList(cli, gvk, rbac.NamespacePermissions)
						respJson, err := json.Marshal(resourceList)
						if err != nil {
							klog.V(0).ErrorS(err, fmt.Sprintf("encountered an error marshalling json for %s", resourceList.GetKind()))
						}

						rw.Header().Add("Content-Type", "application/json")
						_, err = rw.Write(respJson)
						if err != nil {
							klog.V(0).ErrorS(err, "encountered an error writing JSON to client")
						}
					}
				} else { // time to fake the cluster request
					resourceList := getNamespacedResourceList(cli, gvk, rbac.NamespacePermissions)
					respJson, err := json.Marshal(resourceList)
					if err != nil {
						klog.V(0).ErrorS(err, fmt.Sprintf("encountered an error marshalling json for %s", resourceList.GetKind()))
					}

					rw.Header().Add("Content-Type", "application/json")
					_, err = rw.Write(respJson)
					if err != nil {
						klog.V(0).ErrorS(err, "encountered an error writing JSON to client")
					}
				}
			}
		} else {
			direct = true
		}
	}

	return direct
}

func isClusterScopedRequest(url *url.URL) bool {
	path := strings.Trim(url.EscapedPath(), "/")
	klog.V(0).Infof("PATH -- ", path)

	paths := strings.Split(path, "/")
	if paths[0] == "api" {
		if len(paths) >= 5 {
			klog.V(0).Infof("isClusterScoped? -- ", false)
			return false
		}
	} else if paths[0] == "apis" {
		if len(paths) > 5 {
			klog.V(0).Infof("isClusterScoped? -- ", false)
			return false
		}
	}

	klog.V(0).Infof("isClusterScoped? -- ", true)
	return true
}

func isListRequest(url *url.URL) bool {
	path := strings.Trim(url.EscapedPath(), "/")
	klog.V(0).Infof("PATH -- ", path)

	paths := strings.Split(path, "/")
	if paths[0] == "api" {
		if len(paths) == 4 || len(paths) == 6 {
			klog.V(0).Infof("isListRequest? -- ", false)
			return false
		}
	} else if paths[0] == "apis" {
		if len(paths) == 5 || len(paths) == 7 {
			klog.V(0).Infof("isListRequest? -- ", false)
			return false
		}
	}

	klog.V(0).Infof("isListRequest? -- ", true)
	return true
}

func isWatchRequest(url *url.URL) bool {
	out := url.Query().Has("watch")
	klog.V(0).Infof("isWatchRequest? -- ", out)
	return out
}

func isSpecificRequest(url *url.URL) bool {
	out := true
	path := strings.Trim(url.EscapedPath(), "/")
	klog.V(0).Infof("PATH -- ", path)

	paths := strings.Split(path, "/")
	if paths[0] == "api" {
		if isClusterScopedRequest(url) {
			if len(paths) != 4 {
				out = false
			}
		} else {
			if len(paths) != 6 {
				out = false
			}
		}
	} else if paths[0] == "apis" {
		if isClusterScopedRequest(url) {
			if len(paths) != 5 {
				out = false
			}
		} else {
			if len(paths) != 7 {
				out = false
			}
		}
	}

	klog.V(0).Infof("isSpecificRequest? -- ", out)
	return out
}

func gvkFromURL(url *url.URL) schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   groupFromURL(url),
		Version: versionFromURL(url),
		Kind:    kindFromURL(url),
	}
}

func kindFromURL(url *url.URL) string {
	caser := cases.Title(language.AmericanEnglish)
	kind := ""
	path := strings.Trim(url.EscapedPath(), "/")
	klog.V(0).Infof("PATH -- ", path)

	paths := strings.Split(path, "/")
	if paths[0] == "api" {
		if isClusterScopedRequest(url) {
			kind = paths[2]
		} else {
			kind = paths[4]
		}
	} else if paths[0] == "apis" {
		if isClusterScopedRequest(url) {
			kind = paths[3]
		} else {
			kind = paths[5]
		}
	}

	kind = caser.String(strings.TrimSuffix(kind, "s"))
	klog.V(0).Infof("KIND -- ", kind)
	return kind
}

func groupFromURL(url *url.URL) string {
	group := ""
	path := strings.Trim(url.EscapedPath(), "/")
	klog.V(0).Infof("PATH -- ", path)

	paths := strings.Split(path, "/")
	if paths[0] == "api" {
		group = ""
	} else if paths[0] == "apis" {
		group = paths[1]
	}

	klog.V(0).Infof("GROUP -- ", group)
	return group
}

func versionFromURL(url *url.URL) string {
	version := ""
	path := strings.Trim(url.EscapedPath(), "/")
	klog.V(0).Infof("PATH -- ", path)

	paths := strings.Split(path, "/")
	if paths[0] == "api" {
		version = paths[1]
	} else if paths[0] == "apis" {
		version = paths[2]
	}

	klog.V(0).Infof("VERSION -- ", version)
	return version
}

func getNamespacedResourceList(cli client.Client, gvk schema.GroupVersionKind, nsPerms rbac.NamespacedPermissions) *unstructured.UnstructuredList {
	namespaces := []string{}
	// lowercase and pluralized Kind represents the resource
	resource := strings.ToLower(gvk.Kind) + "s"
	for namespace, permissions := range nsPerms {
		if _, ok := permissions[resource]; ok {
			if _, ok := permissions[resource]["*"]; ok { // has all permissions for the resource
				namespaces = append(namespaces, namespace)
			} else if _, ok := permissions[resource]["list"]; ok { // has list permissions for the resource
				namespaces = append(namespaces, namespace)
			}
		}
	}

	// make a request to each namespace in the list for the resource list
	// using unstructured.Unstructured here so we can do it dynamically for the resource

	resourceList := &unstructured.UnstructuredList{}
	resourceList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   gvk.Group,
		Version: gvk.Version,
		// Here we want a list of the objects
		Kind: gvk.Kind + "List",
	})

	for _, ns := range namespaces {
		tempList := &unstructured.UnstructuredList{}
		tempList.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   gvk.Group,
			Version: gvk.Version,
			Kind:    gvk.Kind + "List",
		})

		err := cli.List(context.Background(), tempList, &client.ListOptions{
			Namespace: ns,
		})
		if err != nil {
			klog.V(0).ErrorS(err, fmt.Sprintf("encountered an error getting %s for namespace `%s`", tempList.GetKind(), ns))
			continue
		}

		// Loop through the items and add them to the main list
		for _, item := range tempList.Items {
			resourceList.Items = append(resourceList.Items, item)
		}

		// set the resourceVersion in the event it needs to be used in a watches request
		resourceList.SetResourceVersion(tempList.GetResourceVersion())
	}

	return resourceList
}
