/*
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
*/

package controller

import (
	"context"
	"errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	vpa "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"time"

	"github.com/expr-lang/expr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/stackrox/dynamic-vertical-pod-autoscaler/api/v1alpha1"
)

// DynamicVerticalPodAutoscalerReconciler reconciles a DynamicVerticalPodAutoscaler object
type DynamicVerticalPodAutoscalerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// defaultResult sets the default RequeueAfter.
var defaultResult = ctrl.Result{Requeue: true, RequeueAfter: time.Second * 10}

//+kubebuilder:rbac:groups=autoscaling.stackrox.io,resources=dynamicverticalpodautoscalers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=autoscaling.stackrox.io,resources=dynamicverticalpodautoscalers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=autoscaling.stackrox.io,resources=dynamicverticalpodautoscalers/finalizers,verbs=update
//+kubebuilder:rbac:groups=autoscaling.k8s.io,resources=verticalpodautoscalers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch
//+kubebuilder:rbac:groups=apps,resources=replicasets,verbs=get;list;watch

func (r *DynamicVerticalPodAutoscalerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Ensure that the VerticalPodAutoscaler CRD is installed
	if _, err := r.RESTMapper().KindFor(vpa.SchemeGroupVersion.WithResource("verticalpodautoscalers")); err != nil {
		logger.Error(err, "The VerticalPodAutoscaler CRD is not installed. Please install it before using this controller.")
		return ctrl.Result{}, err
	}

	var obj v1alpha1.DynamicVerticalPodAutoscaler
	if err := r.Get(ctx, req.NamespacedName, &obj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Sanity checks
	if obj.Spec.TargetRef == nil {
		return ctrl.Result{}, errors.New("targetRef is required")
	}
	if len(obj.Spec.TargetRef.Kind) == 0 {
		return ctrl.Result{}, errors.New("targetRef.kind is required")
	}
	if len(obj.Spec.TargetRef.APIVersion) == 0 {
		return ctrl.Result{}, errors.New("targetRef.apiVersion is required")
	}
	if len(obj.Spec.TargetRef.Name) == 0 {
		return ctrl.Result{}, errors.New("targetRef.name is required")
	}
	if len(obj.Spec.Policies) == 0 {
		return ctrl.Result{}, errors.New("conditions is required")
	}

	vpaTarget, err := r.getVPATarget(ctx, obj)
	if client.IgnoreNotFound(err) != nil {
		return ctrl.Result{}, err
	}

	var existingVpa = &vpa.VerticalPodAutoscaler{}
	if err := r.Get(ctx, client.ObjectKey{Name: obj.Name, Namespace: obj.Namespace}, existingVpa); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, err
		}
	}

	env, err := r.getProgramEnv(obj, existingVpa, vpaTarget)
	if err != nil {
		return ctrl.Result{}, err
	}

	var matchedPolicy *v1alpha1.DynamicVerticalPodAutoscalerPolicy
	for i, policy := range obj.Spec.Policies {

		logger.V(5).Info("Checking policy",
			"condition", policy.Condition,
			"index", i,
		)

		if len(policy.Condition) == 0 {
			// When condition is empty, this evaluates to true
			matchedPolicy = &policy
			break
		}

		program, err := expr.Compile(policy.Condition, expr.Env(env))
		if err != nil {
			return ctrl.Result{}, err
		}

		output, err := expr.Run(program, env)
		if err != nil {
			return ctrl.Result{}, err
		}

		if output.(bool) {
			matchedPolicy = &policy
			break
		}
	}

	if matchedPolicy == nil {
		return ctrl.Result{}, errors.New("no matching policy found")
	}

	if matchedPolicy.Skip {
		logger.V(5).Info("Skipping reconciliation")
		return defaultResult, nil
	}

	logger.V(5).Info("Reconciling",
		"policy", matchedPolicy.Condition,
	)

	wantVpaSpec := makeVpaSpec(&obj, &matchedPolicy.VpaSpec)

	// Get or create the VPA object
	foundVPA := &vpa.VerticalPodAutoscaler{}
	if err := r.Get(ctx, client.ObjectKey{Name: obj.Name, Namespace: obj.Namespace}, foundVPA); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, err
		}

		logger.V(5).Info("Creating VerticalPodAutoscaler")

		want := &vpa.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      obj.Name,
				Namespace: obj.Namespace,
			},
			Spec: wantVpaSpec,
		}

		if err := controllerutil.SetControllerReference(&obj, want, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}

		if err := r.Create(ctx, want); err != nil {
			return ctrl.Result{}, err
		}

		obj.Status.VPALastUpdateTime = metav1.NewTime(time.Now().In(time.UTC))
		if err := r.Status().Update(ctx, &obj); err != nil {
			return ctrl.Result{}, err
		}

	} else {
		logger.V(5).Info("Found existing VerticalPodAutoscaler")
		if err := controllerutil.SetControllerReference(&obj, foundVPA, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		if !reflect.DeepEqual(foundVPA.Spec, wantVpaSpec) {
			foundVPA.Spec = wantVpaSpec
			if err := r.Update(ctx, foundVPA); err != nil {
				return ctrl.Result{}, err
			}
			obj.Status.VPALastUpdateTime = metav1.NewTime(time.Now().In(time.UTC))
			if err := r.Status().Update(ctx, &obj); err != nil {
				return ctrl.Result{}, err
			}
		} else {
			logger.V(5).Info("No update needed")
		}
	}

	return defaultResult, nil
}

// getProgramEnv returns the environment available in the expr-lang condition
func (r *DynamicVerticalPodAutoscalerReconciler) getProgramEnv(
	obj v1alpha1.DynamicVerticalPodAutoscaler,
	existingVpa *vpa.VerticalPodAutoscaler,
	vpaTarget *unstructured.Unstructured,
) (map[string]interface{}, error) {

	var objUnstructured = &unstructured.Unstructured{}
	if err := r.Scheme.Convert(&obj, objUnstructured, nil); err != nil {
		return nil, err
	}

	var vpaUnstructured = &unstructured.Unstructured{}
	if existingVpa != nil {
		if err := r.Scheme.Convert(existingVpa, vpaUnstructured, nil); err != nil {
			return nil, err
		}
	}

	env := map[string]interface{}{
		"target": vpaTarget.Object,
		"vpa":    vpaUnstructured.Object,
		"obj":    objUnstructured.Object,
	}

	return env, nil
}

func makeVpaSpec(owner *v1alpha1.DynamicVerticalPodAutoscaler, wantSpec *v1alpha1.VpaSpec) vpa.VerticalPodAutoscalerSpec {
	return vpa.VerticalPodAutoscalerSpec{
		TargetRef:      owner.Spec.TargetRef,
		UpdatePolicy:   wantSpec.UpdatePolicy,
		ResourcePolicy: wantSpec.ResourcePolicy,
		Recommenders:   wantSpec.Recommenders,
	}
}

// findVPATarget finds the VPA target object. Result may be nil
func (r *DynamicVerticalPodAutoscalerReconciler) getVPATarget(ctx context.Context, obj v1alpha1.DynamicVerticalPodAutoscaler) (*unstructured.Unstructured, error) {
	targetGV, err := schema.ParseGroupVersion(obj.Spec.TargetRef.APIVersion)
	if err != nil {
		return nil, err
	}
	targetGvk := targetGV.WithKind(obj.Spec.TargetRef.Kind)

	var target = unstructured.Unstructured{}
	target.SetNamespace(obj.Namespace)
	target.SetGroupVersionKind(targetGvk)
	target.SetName(obj.Spec.TargetRef.Name)
	if err := r.Get(ctx, client.ObjectKeyFromObject(&target), &target); err != nil {
		return nil, err
	}
	return &target, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DynamicVerticalPodAutoscalerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.DynamicVerticalPodAutoscaler{}).
		Owns(&vpa.VerticalPodAutoscaler{}).
		Complete(r)
}
