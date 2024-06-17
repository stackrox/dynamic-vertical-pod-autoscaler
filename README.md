# dynamic-vertical-pod-autoscaler

Use dynamic expressions to manage [VerticalPodAutoscalers](https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler)

## Description

This project is a Kubernetes operator that manages VerticalPodAutoscalers using dynamic expressions.

### Example

```yaml
apiVersion: autoscaling.stackrox.io/v1alpha1
kind: DynamicVerticalPodAutoscaler
metadata:
  name: example
spec:
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: example
  policies:
    - condition: |
        target.metadata.annotations?.["vpa-disabled"] == "true" ?? false
      vpaSpec:
        updatePolicy:
          updateMode: "Off"
    - vpaSpec:
        updatePolicy:
          updateMode: "Auto"
```

The controller will evaluate each policy sequentially, and will apply
the `vpaSpec` defined for the first policy that
evaluates to `true`. If no policy evaluates to `true`, the controller
will throw an error. The absence of a `condition` field is equivalent to
`true`.

The conditions are written with [expr](https://github.com/expr-lang/expr).

There are 3 fields available in the expression script:

1. `target`: The target object of the VPA (Deployment, StatefulSet, etc.)
2. `vpa`: The `VerticalPodAutoscaler` object. May be nil.
3. `obj`: The `DynamicVerticalPodAutoscaler` object.

These objects are passed as a `map[string]interface{}`.
See [sample](./config/samples/_v1alpha1_dynamicverticalpodautoscaler.yaml)
for more example policies.

### `DynamicVerticalPodAutoscalerSpec`

| Field     | Description                      | Type                                   | Required |
|-----------|----------------------------------|----------------------------------------|----------|
| targetRef | The target object of the VPA     | `ObjectReference`                      | Yes      |
| policies  | The list of policies to evaluate | `[]DynamicVerticalPodAutoscalerPolicy` | Yes      |

At least one policy must evaluate to `true`.

### `DynamicVerticalPodAutoscalerPolicy`

| Field     | Description                                              | Type      | Required |
|-----------|----------------------------------------------------------|-----------|----------|
| condition | The condition to evaluate. Empty means `true`            | `string`  | No       |
| vpaSpec   | The VPA spec to apply                                    | `VpaSpec` | No       |
| skip      | Skip reconciliation if the condition evaluates to `true` | `bool`    | No       |

### `VpaSpec`

See the
[VerticalPodAutoscaler](https://github.com/kubernetes/autoscaler/blob/master/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1/types.go)
for the fields available in the `VpaSpec`.

## Getting Started

### Prerequisites

- go version v1.20.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### To Deploy on the cluster

**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=<some-registry>/dynamic-vertical-pod-autoscaler:tag
```

**NOTE:** This image ought to be published in the personal registry you specified.
And it is required to have access to pull the image from the working environment.
Make sure you have the proper permission to the registry if the above commands don’t work.

**Install the `VerticalPodAutoscaler` operator**

```sh
make install-vpa-operator
```

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=<some-registry>/dynamic-vertical-pod-autoscaler:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
> privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```

> **NOTE**: Ensure that the samples has default values to test it out.

### To Uninstall

**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## Contributing

// TODO(user): Add detailed information on how you would like others to contribute to this project

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

