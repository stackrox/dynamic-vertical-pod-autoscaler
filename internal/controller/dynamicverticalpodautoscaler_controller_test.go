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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	autoscaling "k8s.io/api/autoscaling/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	vpa "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	mydomainv1alpha1 "github.com/stackrox/dynamic-vertical-pod-autoscaler/api/v1alpha1"
)

var updateModeOff = vpa.UpdateModeOff
var updateModeInitial = vpa.UpdateModeInitial
var updateModeAuto = vpa.UpdateModeAuto
var updateModeRecreate = vpa.UpdateModeRecreate

var _ = Describe("DynamicVerticalPodAutoscaler Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		obj := &mydomainv1alpha1.DynamicVerticalPodAutoscaler{}

		BeforeEach(func() {
			By("creating a deployment")
			// Create a deployment to be used as the targetRef
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: resourceName, Namespace: "default"},
				Spec: appsv1.DeploymentSpec{
					Replicas: func(i int32) *int32 { return &i }(1),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": resourceName},
					},
					Template: v1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": resourceName}},
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Name:  resourceName,
									Image: "nginx",
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, deployment)).To(Succeed())

			By("creating the custom resource for the Kind DynamicVerticalPodAutoscaler")
			err := k8sClient.Get(ctx, typeNamespacedName, obj)
			if err != nil && errors.IsNotFound(err) {
				resource := &mydomainv1alpha1.DynamicVerticalPodAutoscaler{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: mydomainv1alpha1.DynamicVerticalPodAutoscalerSpec{
						TargetRef: &autoscaling.CrossVersionObjectReference{
							Kind:       "Deployment",
							Name:       resourceName,
							APIVersion: "apps/v1",
						},
						Policies: []mydomainv1alpha1.DynamicVerticalPodAutoscalerPolicy{
							{
								Condition: "false",
								VpaSpec: mydomainv1alpha1.VpaSpec{
									UpdatePolicy: &vpa.PodUpdatePolicy{
										UpdateMode: &updateModeRecreate,
									},
								},
							}, {
								Condition: "true",
								VpaSpec: mydomainv1alpha1.VpaSpec{
									UpdatePolicy: &vpa.PodUpdatePolicy{
										UpdateMode: &updateModeOff,
									},
								},
							},
						},
					},
					// TODO(user): Specify other spec details if needed.
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &mydomainv1alpha1.DynamicVerticalPodAutoscaler{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance DynamicVerticalPodAutoscaler")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &DynamicVerticalPodAutoscalerReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			reconcileReq := reconcile.Request{NamespacedName: typeNamespacedName}

			_, err := controllerReconciler.Reconcile(ctx, reconcileReq)
			Expect(err).NotTo(HaveOccurred())

			By("Creating a VerticalPodAutoscaler")
			vpaResource := &vpa.VerticalPodAutoscaler{}
			err = k8sClient.Get(ctx, typeNamespacedName, vpaResource)
			Expect(err).NotTo(HaveOccurred())

			By("Applying the right condition")
			Expect(vpaResource.Spec.TargetRef.Name).To(Equal(resourceName))
			Expect(vpaResource.Spec.UpdatePolicy.UpdateMode).To(Equal(&updateModeOff))

			By("Updating the status of the DynamicVerticalPodAutoscaler")
			updatedResource := &mydomainv1alpha1.DynamicVerticalPodAutoscaler{}
			err = k8sClient.Get(ctx, typeNamespacedName, updatedResource)
			Expect(err).NotTo(HaveOccurred())
			Expect(updatedResource.Status.VPALastUpdateTime).NotTo(Equal(0))

			By("Changing the condition")
			updatedResource.Spec.Policies[0].Condition = "true"
			Expect(k8sClient.Update(ctx, updatedResource)).To(Succeed())

			By("Reconciling the updated resource")
			_, err = controllerReconciler.Reconcile(ctx, reconcileReq)
			Expect(err).NotTo(HaveOccurred())

			By("Verifying the VerticalPodAutoscaler is updated")
			err = k8sClient.Get(ctx, typeNamespacedName, vpaResource)
			Expect(err).NotTo(HaveOccurred())
			Expect(vpaResource.Spec.UpdatePolicy.UpdateMode).To(Equal(&updateModeRecreate))

		})

	})
})
