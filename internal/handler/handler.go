package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/everettraven/rbac-proxy-poc/internal/rbac"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

// HandleRequest will handle the processing of a proxy request. It accepts a http.ResponseWriter,
// http.Request, and an RBACWatcher. It will return a bool that represents whether or not the request
// should continue to be proxied directly to the Kubernetes API server. It returns true if the request
// should continue and false if the request has been handled.
// This function handles the following scenarios:
// 1. A request for a specific resource - continue to proxy to Kubernetes API
// 2. A request to list/watch resources in a specific namespace - continue to proxy to Kubernetes API
// 3. A request to list resources at the cluster level (has permissions) - continue to proxy to Kubernetes API
// 4. A request to list resources at the cluster level (does NOT have permissions) - handle the request and do NOT continue to proxy
// TODO: 5. A request to watch resources at the cluster level (has permissions) - continue to proxy to Kubernetes API
// TODO: 6. A request to watch resources at the cluster level (does NOT have permissions) - handle the request and do NOT continue to proxy
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
						resourceList := getNamespacedResourceList(cli, gvk, rbac.NamespacePermissions, "list")
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
					resourceList := getNamespacedResourceList(cli, gvk, rbac.NamespacePermissions, "list")
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
