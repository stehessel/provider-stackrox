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

// ClusterParameters are the configurable fields of a Cluster.
type ClusterParameters struct {
	// +kubebuilder:default=true
	// +kubebuilder:validation:Optional
	AdmissionController bool `json:"admissionController"`

	// +kubebuilder:default=false
	// +kubebuilder:validation:Optional
	AdmissionControllerEvents bool `json:"admissionControllerEvents"`

	// +kubebuilder:default=false
	// +kubebuilder:validation:Optional
	AdmissionControllerUpdates bool `json:"admissionControllerUpdates"`

	CentralAPIEndpoint string `json:"centralAPIEndpoint"`

	// +kubebuilder:default=EBPF
	// +kubebuilder:validation:Enum=UNSET_COLLECTION;NO_COLLECTION;KERNEL_MODULE;EBPF
	// +kubebuilder:validation:Optional
	CollectionMethod string `json:"collectionMethod"`

	// +kubebuilder:default=registry.redhat.io/advanced-cluster-security/rhacs-collector-rhel8
	// +kubebuilder:validation:Optional
	CollectorImage string `json:"collectorImage"`

	// +kubebuilder:validation:Optional
	Labels map[string]string `json:"labels"`

	// +kubebuilder:default=registry.redhat.io/advanced-cluster-security/rhacs-main-rhel8
	// +kubebuilder:validation:Optional
	MainImage string `json:"mainImage"`

	Name string `json:"name"`

	// +kubebuilder:default=true
	// +kubebuilder:validation:Optional
	SlimCollector bool `json:"slimCollector"`

	// +kubebuilder:default=true
	// +kubebuilder:validation:Optional
	Tolerations bool `json:"tolerations"`

	// +kubebuilder:default=GENERIC_CLUSTER
	// +kubebuilder:validation:Enum=GENERIC_CLUSTER;KUBERNETES_CLUSTER;OPENSHIFT_CLUSTER;OPENSHIFT4_CLUSTER
	// +kubebuilder:validation:Optional
	Type string `json:"type"`
}

// SensorDeployment contains information about the last Sensor connected to the cluster.
type SensorDeployment struct {
	AppNamespace        string `json:"appNamespace,omitempty"`
	AppNamespaceID      string `json:"appNamespaceID,omitempty"`
	AppServiceAccountID string `json:"appServiceAccountID,omitempty"`
	DefaultNamespaceID  string `json:"defaultNamespaceID,omitempty"`
	K8SNodeName         string `json:"k8sNodeName,omitempty"`
	SystemNamespaceID   string `json:"systemNamespaceID,omitempty"`
}

// ClusterObservation are the observable fields of a Cluster.
type ClusterObservation struct {
	AdmissionController bool `json:"admissionController,omitempty"`

	AdmissionControllerEvents bool `json:"admissionControllerEvents,omitempty"`

	AdmissionControllerUpdates bool `json:"admissionControllerUpdates,omitempty"`

	CentralAPIEndpoint string `json:"centralAPIEndpoint,omitempty"`

	// +kubebuilder:validation:Enum=UNSET_COLLECTION;NO_COLLECTION;KERNEL_MODULE;EBPF
	CollectionMethod string `json:"collectionMethod,omitempty"`

	CollectorImage string `json:"collectorImage,omitempty"`

	ID string `json:"id,omitempty"`

	InitBundleID string `json:"initBundleID,omitempty"`

	Labels map[string]string `json:"labels,omitempty"`

	MainImage string `json:"mainImage,omitempty"`

	// +kubebuilder:validation:Enum=MANAGER_TYPE_UNKNOWN;MANAGER_TYPE_MANUAL;MANAGER_TYPE_HELM_CHART;MANAGER_TYPE_KUBERNETES_OPERATOR
	ManagedBy string `json:"managedBy,omitempty"`

	MostRecentSensor SensorDeployment `json:"mostRecentSensor,omitempty"`

	Name string `json:"name,omitempty"`

	SlimCollector bool `json:"slimCollector,omitempty"`

	Tolerations bool `json:"tolerations,omitempty"`

	// +kubebuilder:validation:Enum=GENERIC_CLUSTER;KUBERNETES_CLUSTER;OPENSHIFT_CLUSTER;OPENSHIFT4_CLUSTER
	Type string `json:"type,omitempty"`
}

// A ClusterSpec defines the desired state of a Cluster.
type ClusterSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       ClusterParameters `json:"forProvider"`
}

// A ClusterStatus represents the observed state of a Cluster.
type ClusterStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          ClusterObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A Cluster is an example API type.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,stackrox}
type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterSpec   `json:"spec"`
	Status ClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterList contains a list of Cluster
type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Cluster `json:"items"`
}

// Cluster type metadata.
var (
	ClusterKind             = reflect.TypeOf(Cluster{}).Name()
	ClusterGroupKind        = schema.GroupKind{Group: Group, Kind: ClusterKind}.String()
	ClusterKindAPIVersion   = ClusterKind + "." + SchemeGroupVersion.String()
	ClusterGroupVersionKind = SchemeGroupVersion.WithKind(ClusterKind)
)

func init() {
	SchemeBuilder.Register(&Cluster{}, &ClusterList{})
}
