# RBAC Proxy PoC
PoC for using a sidecar proxy to scope a Descoped Operator using RBAC.

## Assumptions
1. Operator's `ServiceAccount` has cluster level permissions to `get`, `list`, and `watch` the `ClusterRole`, `Role`, `ClusterRoleBinding`, `RoleBinding` resources of the `rbac.authorization.k8s.io` api group

## Functionality Expectations
- If a request for a specific resource is received:
    - The request is proxied directly to the Kubernetes API
- If a request for a list/watch of resources in a specific namespace is received:
    - The request is proxied directly to the Kubernetes API
- If a request for a list/watch of resources at the cluster level is received:
    - If the operator has permissions to list/watch the requested resource at the cluster level
        - The request is proxied directly to the Kubernetes API
    - If the operator does NOT have permissions to list/watch the requested resource at the cluster level
        - The proxy gets a list/watch for each of the namespaces on the cluster that the operator has list/watch permissions on for the requested resource and merges it into one resource list that is returned as the response. This makes it look to the operator as if it has a cluster-wide view of the resource it requested.

## Testing the proxy as a sidecar
1. Build the image with: 
    ```sh
    docker build -t <tag> .
    ```
2. Create a KinD cluster with:
    ```sh
    kind create cluster
    ```
3. Load the image to the KinD cluster with:
    ```sh
    kind load docker-image <tag>
    ```
4. Modify the `demo.yaml` file to have the sidecar pod use your image tag
5. Apply the `demo.yaml` file with:
    ```sh
    kubectl apply -f demo.yaml
    ```
6. Run `curl` requests against the sidecar proxy using:
    ```sh
    kubectl exec -it rbac-sidecar -- curl 127.0.0.1:8001/...
    ```

**NOTE**: If you don't want to build the image yourself, for testing you can use the image `bpalmer/rbac-proxy-poc:latest` and skip steps 1, 3, and 4

When following the above steps, the only permissions that the `ServiceAccount` used by the `rbac-sidecar` pod should have is the permissions to `get`, `list`, and `watch` pods in the `default` namespace and the cluster level permissions mentioned in the **Assumptions** section above.

## Demo Steps
1. `kind create cluster`
2. `cat demo.yaml | bat -l yaml` (show the demo manifest)
3. `kubectl apply -f demo.yaml`
4. `watch -n 5 kubectl get pods` (wait for pod to finish starting up)
5. `kubectl get pods -A` (show pods in all namespaces)
6. `kubectl exec -it rbac-sidecar -- curl 127.0.0.1:8001/api/v1/pods | jq` (show that proxy filters only to the allowed namespace)
6.1. `kubectl exec -it rbac-sidecar -- curl 127.0.0.1:8001/api/v1/pods | jq '.items | length'` (show that we only have 1 pod)
7. `kubectl exec -it rbac-sidecar -- curl 127.0.0.1:8001/api/v1/namespaces/default/pods | jq` (show that we can get the pods in the default namespace)
8. `kubectl exec -it rbac-sidecar -- curl 127.0.0.1:8001/api/v1/namespaces/kube-system/pods | jq` (show that we can NOT get the pods in the kube-system namespace)
9. `kubectl exec -it rbac-sidecar -- curl 127.0.0.1:8001/apis/batch/v1/jobs | jq` (show that we get an empty list when we don't have permissions on the resource anywhere)
10. `kubectl exec -it rbac-sidecar -- curl 127.0.0.1:8001/apis/batch/v1/namespaces/default/jobs | jq` (show that we get a forbidden since we don't have permissions)
11. `kubectl exec -it rbac-sidecar -- curl 127.0.0.1:8001/apis/batch/v1/namespaces/default/jobs/job-name | jq` (show forbidden)
12. `kubectl exec -it rbac-sidecar -- curl 127.0.0.1:8001/apis/batch/v1/jobs/job-name | jq` (show forbidden)

## Demo GIF
![demo gif](proxy-poc-demo.gif)
