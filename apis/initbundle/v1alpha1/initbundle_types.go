/*
Copyright 2022 The Crossplane Authors.

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
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// Attributes defines a map of user attributes.
type Attributes map[string]string

// ImpactedCluster represents a secured cluster impacted by an init bundle.
type ImpactedCluster struct {
	// ID of the cluster.
	ID string `json:"id"`

	// Name of the cluster.
	Name string `json:"name"`
}

// User represents the actor that created the init bundle.
type User struct {
	// Attributes of the user.
	Attributes Attributes `json:"attributes"`

	// AuthProviderID which is associated with the user.
	AuthProviderID string `json:"authProviderID"`

	// ID of the user.
	ID string `json:"id"`
}

// InitBundleParameters are the configurable fields of a InitBundle.
type InitBundleParameters struct {
	// Name of the init bundle.
	Name string `json:"name"`
}

// InitBundleObservation are the observable fields of a InitBundle.
type InitBundleObservation struct {
	// CreatedAt timestamp of the init bundle.
	CreatedAt metav1.Time `json:"createdAt,omitempty"`

	// CreatedBy timestamp of the init bundle.
	CreatedBy User `json:"createdBy,omitempty"`

	// ExpiresAt timestamp of the init bundle.
	ExpiresAt metav1.Time `json:"expiresAt,omitempty"`

	// ID of the init bundle.
	ID string `json:"id,omitempty"`

	// ImpactedClusters defines a list of secured clusters impacted by the init bundle.
	ImpactedClusters []ImpactedCluster `json:"impactedClusters,omitempty"`

	// Name of the init bundle.
	Name string `json:"name,omitempty"`
}

// A InitBundleSpec defines the desired state of a InitBundle.
type InitBundleSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       InitBundleParameters `json:"forProvider"`
}

// A InitBundleStatus represents the observed state of a InitBundle.
type InitBundleStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          InitBundleObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A InitBundle is an example API type.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,redhat}
type InitBundle struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InitBundleSpec   `json:"spec"`
	Status InitBundleStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// InitBundleList contains a list of InitBundle
type InitBundleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InitBundle `json:"items"`
}

// InitBundle type metadata.
var (
	InitBundleKind             = reflect.TypeOf(InitBundle{}).Name()
	InitBundleGroupKind        = schema.GroupKind{Group: Group, Kind: InitBundleKind}.String()
	InitBundleKindAPIVersion   = InitBundleKind + "." + SchemeGroupVersion.String()
	InitBundleGroupVersionKind = SchemeGroupVersion.WithKind(InitBundleKind)
)

func init() {
	SchemeBuilder.Register(&InitBundle{}, &InitBundleList{})
}
