package main

import (
	"fmt"
	"os"

	"github.com/everettraven/rbac-proxy-poc/internal/proxy"
	"github.com/everettraven/rbac-proxy-poc/pkg/scoped"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

const (
	port          = 8001
	staticPrefix  = "/static/"
	apiPrefix     = "/"
	address       = "127.0.0.1"
	acceptPaths   = proxy.DefaultPathAcceptRE
	rejectPaths   = proxy.DefaultPathRejectRE
	acceptHosts   = proxy.DefaultHostAcceptRE
	rejectMethods = proxy.DefaultMethodRejectRE
	staticDir     = ""
)

func main() {
	fmt.Println("RBAC Proxy!")

	fmt.Println("Getting Pods!")
	cli := dynamic.NewForConfigOrDie(config.GetConfigOrDie())
	gvr := corev1.SchemeGroupVersion.WithResource("pods")
	lw := scoped.NewScopedListerWatcher(cli, gvr)
	pods, err := lw.List(metav1.ListOptions{})
	if err != nil {
		fmt.Println("ERROR --> ", err)
		os.Exit(1)
	}

	fmt.Println("Got some Pods!")
	ulpods := pods.(*unstructured.UnstructuredList)

	for _, pod := range ulpods.Items {
		fmt.Println("Got Pod --> ", pod.GetName())
	}

	fmt.Println("-----------------------------------")
	fmt.Println("Watching Pods!")
	wc := make(chan string)
	watcher, err := lw.Watch(metav1.ListOptions{})
	if err != nil {
		fmt.Println("ERROR --> ", err)
		os.Exit(2)
	}

	go func() {
		for event := range watcher.ResultChan() {
			metaObj := event.Object.(metav1.Object)
			wc <- fmt.Sprintf("%s - `%s` in namespace `%s`", event.Type, metaObj.GetName(), metaObj.GetNamespace())
		}
	}()

	for {
		out := <-wc
		fmt.Println(out)
	}

	// err := RunProxy()
	// if err != nil {
	// 	fmt.Println("ERROR -- ", err)
	// }
}

// func RunProxy() error {
// 	// Create an informer
// 	cfg := config.GetConfigOrDie()
// 	ctx := context.TODO()
// 	defer ctx.Done()

// 	watcher := rbac.NewRBACWatcher("rbac-sa")
// 	watcher.Initialize(ctx, cfg)

// 	go watcher.Start(ctx)

// 	filter := &proxy.FilterServer{
// 		AcceptPaths:        proxy.MakeRegexpArrayOrDie(acceptPaths),
// 		RejectPaths:        proxy.MakeRegexpArrayOrDie(rejectPaths),
// 		AcceptHosts:        proxy.MakeRegexpArrayOrDie(acceptHosts),
// 		RejectMethods:      proxy.MakeRegexpArrayOrDie(rejectMethods),
// 		PermissionsWatcher: watcher,
// 	}

// 	keepalive, _ := time.ParseDuration("500ms")
// 	appendServerPath := false
// 	server, err := proxy.NewServer(staticDir, apiPrefix, staticPrefix, filter, cfg, keepalive, appendServerPath)

// 	if err != nil {
// 		return err
// 	}

// 	var l net.Listener

// 	l, err = server.Listen(address, port)
// 	if err != nil {
// 		return err
// 	}
// 	fmt.Fprintf(os.Stdout, "Starting to serve on %s\n", l.Addr().String())
// 	return server.ServeOnListener(l)
// }

/*
	Notes on the RBAC Proxy:
	---
	Assumptions:
	- Has rights to read all RBAC on the cluster

	Solution to try:
	- Direct proxy to kube api and handle specific cases when an error occurs
		- When an error happens could build a new result based on permissions and return that.
		- A request has to be made first to see if there will be an error

	Solution Pseudocode:
	Create an RBAC Informer to keep RBAC info up to date
	When a Request is received
	If the Request is of type `watch` {
		Create HTTP streaming response
		Watch for RBAC updates
		Push resource updates to HTTP Stream based on RBAC and resource changes
	}
	Else If the Request is of type `list` {
		If Request is namespaced {
			Direct Proxy
			Return any errors from API server
		}
		Else If request is cluster scoped {
			If ClusterRole binded for resource {
				Direct Proxy
				Return any errors from API server
			}
			Else (in this case only allowed for specific namespaces) {
				Use RBAC information to get namespaces allowed
				Loop through allowed namespaces and get resources
				Create what looks like a cluster level response by appending all resource lists
			}
		}
	}
	Else {
		Direct Proxy
		Return any errors from API server
	}
*/
