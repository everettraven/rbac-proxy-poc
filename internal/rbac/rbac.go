package rbac

import (
	"context"
	"fmt"
	"strings"

	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
	crcache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type RBACWatcher struct {
	ServiceAccountName   string
	ClusterPermissions   Permissions
	NamespacePermissions NamespacedPermissions
	cache                crcache.Cache
	cli                  client.Client
}

type Permissions map[string]map[string]interface{}

type NamespacedPermissions map[string]Permissions

func NewRBACWatcher(sa string) *RBACWatcher {
	return &RBACWatcher{
		ServiceAccountName:   sa,
		ClusterPermissions:   Permissions{},
		NamespacePermissions: NamespacedPermissions{},
	}
}

func (w *RBACWatcher) Initialize(ctx context.Context, cfg *rest.Config) error {
	var err error
	opts := crcache.Options{}
	w.cli, err = client.New(cfg, client.Options{})
	if err != nil {
		return fmt.Errorf("encountered an error creating client: %w", err)
	}
	w.cache, err = crcache.New(cfg, opts)
	if err != nil {
		return fmt.Errorf("encountered an error creating cache: %w", err)
	}

	crbInformer, err := w.cache.GetInformerForKind(ctx, schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "ClusterRoleBinding"})
	if err != nil {
		return fmt.Errorf("encountered an error getting informer for ClusterRoleBinding: %w", err)
	}

	rbInformer, err := w.cache.GetInformerForKind(ctx, schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "RoleBinding"})
	if err != nil {
		return fmt.Errorf("encountered an error getting informer for RoleBinding: %w", err)
	}

	crbInformer.AddEventHandler(w.clusterRoleBindingHandler())
	rbInformer.AddEventHandler(w.roleBindingHandler())
	return nil
}

func (w *RBACWatcher) Start(ctx context.Context) error {
	return w.cache.Start(ctx)
}

// TODO: Finish this handler
func (w *RBACWatcher) clusterRoleBindingHandler() cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			crb := obj.(*rbac.ClusterRoleBinding)
			for _, sub := range crb.Subjects {
				if sub.Kind == "ServiceAccount" && sub.Name == w.ServiceAccountName {
					perms := getPermissionsForClusterRoleBinding(w.cli, crb)
					w.addClusterPerms(perms)
					klog.V(0).Infof("Cluster Permissions after add -- ", w.ClusterPermissions)
					break
				}
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldCrb := oldObj.(*rbac.ClusterRoleBinding)
			var oldPerms Permissions
			newCrb := newObj.(*rbac.ClusterRoleBinding)
			var newPerms Permissions
			hadSA := false
			hasSA := false

			for _, sub := range oldCrb.Subjects {
				if sub.Kind == "ServiceAccount" && sub.Name == w.ServiceAccountName {
					oldPerms = getPermissionsForClusterRoleBinding(w.cli, oldCrb)
					hadSA = true
					break
				}
			}

			for _, sub := range newCrb.Subjects {
				if sub.Kind == "ServiceAccount" && sub.Name == w.ServiceAccountName {
					newPerms = getPermissionsForClusterRoleBinding(w.cli, oldCrb)
					hasSA = true
					break
				}
			}

			// Only change that can be made is that the ServiceAccount could be removed from the subjects of a binding
			if hadSA && !hasSA { // SA was removed
				w.deleteClusterPerms(oldPerms)
				klog.V(0).Infof("Cluster Permissions after update -- ", w.ClusterPermissions)
			} else if !hadSA && hasSA { // SA was added
				w.addClusterPerms(newPerms)
				klog.V(0).Infof("Cluster Permissions after update -- ", w.ClusterPermissions)
			}

		},
		DeleteFunc: func(obj interface{}) {
			crb := obj.(*rbac.ClusterRoleBinding)
			for _, sub := range crb.Subjects {
				if sub.Kind == "ServiceAccount" && sub.Name == w.ServiceAccountName {
					perms := getPermissionsForClusterRoleBinding(w.cli, crb)
					w.deleteClusterPerms(perms)
					klog.V(0).Infof("Cluster Permissions after delete -- ", w.ClusterPermissions)
					break
				}
			}
		},
	}
}

func getPermissionsForClusterRoleBinding(cli client.Client, crb *rbac.ClusterRoleBinding) Permissions {
	perms := Permissions{}
	cr := &rbac.ClusterRole{}
	err := cli.Get(context.Background(), client.ObjectKey{Name: crb.RoleRef.Name}, cr)
	if err != nil {
		klog.V(0).Infof(fmt.Sprintf("encountered an error attempting to get ClusterRole with name: %s", &crb.RoleRef.Name))
	}
	if len(cr.Rules) > 0 {
		klog.V(0).Infof(fmt.Sprintf("processing ClusterRole %s", cr.Name))

		for _, rule := range cr.Rules {
			verbsMap := make(map[string]interface{})
			for _, verb := range rule.Verbs {
				verbsMap[verb] = 0
			}
			for _, res := range rule.Resources {
				klog.V(0).Infof(fmt.Sprintf("ClusterRole `%s` sets resource `%s` with verbs `%s`", cr.Name, res, strings.Join(rule.Verbs, ",")))
				perms[res] = verbsMap
			}
		}
	}

	klog.V(0).Infof("PERMS -- ", perms)
	return perms
}

func (w *RBACWatcher) addClusterPerms(perms Permissions) {
	for key, value := range perms {
		if _, ok := w.ClusterPermissions[key]; ok {
			for k, v := range value {
				if _, ok := w.ClusterPermissions[key][k]; !ok {
					w.ClusterPermissions[key][k] = v
				}
			}
		} else {
			w.ClusterPermissions[key] = value
		}
	}
}

func (w *RBACWatcher) deleteClusterPerms(perms Permissions) {
	for key, value := range perms {
		if _, ok := w.ClusterPermissions[key]; ok {
			for k, _ := range value {
				delete(w.ClusterPermissions[key], k)
			}
		}
	}
}

// TODO: Open Question: Do we need to watch the ClusterRoles and Roles that are applied to the SA via a binding with informers in the case that the roles are modified?
// - For now keep the scope to the bindings and then see about adding the more granular functionality

func (w *RBACWatcher) roleBindingHandler() cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			rb := obj.(*rbac.RoleBinding)
			for _, sub := range rb.Subjects {
				if sub.Kind == "ServiceAccount" && sub.Name == w.ServiceAccountName {
					perms := getPermissionsForRoleBinding(w.cli, rb)
					w.addNamespacedPerms(rb.Namespace, perms)
					klog.V(0).Infof("Namespace Permissions after add -- ", w.NamespacePermissions)
					break
				}
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldRb := oldObj.(*rbac.RoleBinding)
			var oldPerms Permissions
			newRb := newObj.(*rbac.RoleBinding)
			var newPerms Permissions
			hadSA := false
			hasSA := false

			for _, sub := range oldRb.Subjects {
				if sub.Kind == "ServiceAccount" && sub.Name == w.ServiceAccountName {
					oldPerms = getPermissionsForRoleBinding(w.cli, oldRb)
					hadSA = true
					break
				}
			}

			for _, sub := range newRb.Subjects {
				if sub.Kind == "ServiceAccount" && sub.Name == w.ServiceAccountName {
					newPerms = getPermissionsForRoleBinding(w.cli, oldRb)
					hasSA = true
					break
				}
			}

			// Only change that can be made is that the ServiceAccount could be removed from the subjects of a binding
			if hadSA && !hasSA { // SA was removed
				w.deleteNamespacedPerms(oldRb.Namespace, oldPerms)
				klog.V(0).Infof("Namespace Permissions after update -- ", w.NamespacePermissions)
			} else if !hadSA && hasSA { // SA was added
				w.addNamespacedPerms(newRb.Namespace, newPerms)
				klog.V(0).Infof("Namespace Permissions after update -- ", w.NamespacePermissions)
			}

		},
		DeleteFunc: func(obj interface{}) {
			rb := obj.(*rbac.RoleBinding)
			for _, sub := range rb.Subjects {
				if sub.Kind == "ServiceAccount" && sub.Name == w.ServiceAccountName {
					perms := getPermissionsForRoleBinding(w.cli, rb)
					w.deleteNamespacedPerms(rb.Namespace, perms)
					klog.V(0).Infof("Namespace Permissions after delete -- ", w.NamespacePermissions)
					break
				}
			}
		},
	}
}

func getPermissionsForRoleBinding(cli client.Client, rb *rbac.RoleBinding) Permissions {
	perms := Permissions{}
	role := &rbac.Role{}
	err := cli.Get(context.Background(), client.ObjectKey{Name: rb.RoleRef.Name, Namespace: rb.Namespace}, role)
	if err != nil {
		klog.V(0).Infof(fmt.Sprintf("encountered an error attempting to get Role with name: %s", rb.RoleRef.Name))
		return nil
	}
	if len(role.Rules) > 0 {
		klog.V(0).Infof(fmt.Sprintf("processing Role %s", role.Name))
		for _, rule := range role.Rules {
			verbsMap := make(map[string]interface{})
			for _, verb := range rule.Verbs {
				verbsMap[verb] = 0
			}
			for _, res := range rule.Resources {
				klog.V(0).Infof(fmt.Sprintf("Role `%s` sets resource `%s` with verbs `%s`", role.Name, res, strings.Join(rule.Verbs, ",")))
				perms[res] = verbsMap
			}
		}
	}

	klog.V(0).Infof("PERMS -- ", perms)
	return perms
}

func (w *RBACWatcher) addNamespacedPerms(namespace string, perms Permissions) {
	if _, ok := w.NamespacePermissions[namespace]; ok {
		for key, value := range perms {
			if _, ok := w.NamespacePermissions[namespace][key]; ok {
				for k, v := range value {
					if _, ok := w.NamespacePermissions[namespace][key][k]; !ok {
						w.NamespacePermissions[namespace][key][k] = v
					}
				}
			} else {
				w.NamespacePermissions[namespace][key] = value
			}
		}
	} else {
		w.NamespacePermissions[namespace] = perms
	}
}

func (w *RBACWatcher) deleteNamespacedPerms(namespace string, perms Permissions) {
	if _, ok := w.NamespacePermissions[namespace]; ok {
		for key, value := range perms {
			if _, ok := w.NamespacePermissions[namespace][key]; ok {
				for k, _ := range value {
					delete(w.NamespacePermissions[namespace][key], k)
				}
			}
		}
	}
}
