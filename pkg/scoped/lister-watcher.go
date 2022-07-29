package scoped

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
)

type ScopedListerWatcher struct {
	cli dynamic.Interface
	gvr schema.GroupVersionResource
}

func NewScopedListerWatcher(cli dynamic.Interface, gvr schema.GroupVersionResource) *ScopedListerWatcher {
	return &ScopedListerWatcher{
		cli: cli,
		gvr: gvr,
	}
}

func (sl *ScopedListerWatcher) List(options metav1.ListOptions) (runtime.Object, error) {
	canClusterList, err := canClusterVerbResource(sl.cli, sl.gvr, "list")
	if err != nil {
		return nil, err
	}

	if canClusterList {
		return sl.cli.Resource(sl.gvr).List(context.TODO(), options)
	} else {
		permittedNs, err := getNamespacesForVerbResource(sl.cli, sl.gvr, "list")
		if err != nil {
			return nil, err
		}

		return listResourcesForNamespaces(sl.cli, sl.gvr, permittedNs, options)
	}
}

func (sl *ScopedListerWatcher) Watch(options metav1.ListOptions) (watch.Interface, error) {
	canClusterList, err := canClusterVerbResource(sl.cli, sl.gvr, "watch")
	if err != nil {
		return nil, err
	}

	if canClusterList {
		return sl.cli.Resource(sl.gvr).Watch(context.TODO(), options)
	} else {
		permittedNs, err := getNamespacesForVerbResource(sl.cli, sl.gvr, "watch")
		if err != nil {
			return nil, err
		}

		return watchResourcesForNamespaces(sl.cli, sl.gvr, permittedNs, options)
	}
}
