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

package v1alpha1

import (
	autoscaling "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	vpa "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
)

// DynamicVerticalPodAutoscalerSpec defines the desired state of DynamicVerticalPodAutoscaler
type DynamicVerticalPodAutoscalerSpec struct {
	TargetRef *autoscaling.CrossVersionObjectReference `json:"targetRef,omitempty"`
	Policies  []DynamicVerticalPodAutoscalerPolicy     `json:"policies,omitempty"`
}

type DynamicVerticalPodAutoscalerPolicy struct {
	Condition string  `json:"condition,omitempty"`
	Skip      bool    `json:"skip,omitempty"`
	VpaSpec   VpaSpec `json:"vpaSpec,omitempty"`
}

type VpaSpec struct {
	// Describes the rules on how changes are applied to the pods.
	// If not specified, all fields in the `PodUpdatePolicy` are set to their
	// default values.
	// +optional
	UpdatePolicy *vpa.PodUpdatePolicy `json:"updatePolicy,omitempty" protobuf:"bytes,1,opt,name=updatePolicy"`

	// Controls how the autoscaler computes recommended resources.
	// The resource policy may be used to set constraints on the recommendations
	// for individual containers.
	// If any individual containers need to be excluded from getting the VPA recommendations, then
	// it must be disabled explicitly by setting mode to "Off" under containerPolicies.
	// If not specified, the autoscaler computes recommended resources for all containers in the pod,
	// without additional constraints.
	// +optional
	ResourcePolicy *vpa.PodResourcePolicy `json:"resourcePolicy,omitempty" protobuf:"bytes,2,opt,name=resourcePolicy"`

	// Recommender responsible for generating recommendation for this object.
	// List should be empty (then the default recommender will generate the
	// recommendation) or contain exactly one recommender.
	// +optional
	Recommenders []*vpa.VerticalPodAutoscalerRecommenderSelector `json:"recommenders,omitempty" protobuf:"bytes,3,opt,name=recommenders"`
}

// DynamicVerticalPodAutoscalerStatus defines the observed state of DynamicVerticalPodAutoscaler
type DynamicVerticalPodAutoscalerStatus struct {
	// The last time we updated the VerticalPodAutoscaler resource.
	// +optional
	VPALastUpdateTime metav1.Time `json:"vpaLastUpdateTime,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// DynamicVerticalPodAutoscaler is the Schema for the dynamicverticalpodautoscalers API
type DynamicVerticalPodAutoscaler struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DynamicVerticalPodAutoscalerSpec   `json:"spec,omitempty"`
	Status DynamicVerticalPodAutoscalerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DynamicVerticalPodAutoscalerList contains a list of DynamicVerticalPodAutoscaler
type DynamicVerticalPodAutoscalerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DynamicVerticalPodAutoscaler `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DynamicVerticalPodAutoscaler{}, &DynamicVerticalPodAutoscalerList{})
}
