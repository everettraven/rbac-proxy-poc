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