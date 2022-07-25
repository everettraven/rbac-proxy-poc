package rbac

import (
	"context"
	"fmt"
	"strings"

	rbac "k8s.io/api/rbac/v1"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// getPermissionsForClusterRoleBinding is a helper function that will
// fetch the Permissions for a given ClusterRoleBinding resource. It accepts
// a client.Client and rbac.ClusterRoleBinding as parameters and returns a Permissions
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

// getPermissionsForRoleBinding is a helper function that will
// fetch the Permissions for a given RoleBinding resource. It accepts
// a client.Client and rbac.RoleBinding as parameters and returns a Permissions
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
