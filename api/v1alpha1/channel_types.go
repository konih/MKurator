package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ChannelType is the IBM MQ channel object type to manage.
// +kubebuilder:validation:Enum=svrconn
type ChannelType string

const (
	// ChannelTypeSvrconn is a server-connection channel (inbound clients).
	ChannelTypeSvrconn ChannelType = "svrconn"
)

// ChannelFinalizer ensures the MQ channel is deleted before the CR is removed.
const ChannelFinalizer = "messaging.mkurator.dev/channel"

// ChannelSpec defines a channel to maintain on a referenced queue manager.
type ChannelSpec struct {
	// ConnectionRef names a QueueManagerConnection in the same namespace.
	// +kubebuilder:validation:Required
	ConnectionRef LocalObjectReference `json:"connectionRef"`

	// ChannelName is the IBM MQ channel name (e.g. ORDERS.APP).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	ChannelName string `json:"channelName"`

	// Type is the channel kind to define. Only svrconn is reconciled in v1alpha1.
	// +kubebuilder:default=svrconn
	// +optional
	Type ChannelType `json:"type,omitempty"`

	// Attributes map to MQSC parameters (lowercase keys in mqweb runCommandJSON).
	// Drift-checked vs define-only keys: docs/ATTRIBUTE_RECONCILIATION.md.
	// +optional
	Attributes map[string]string `json:"attributes,omitempty"`

	// Suspend pauses MQ reconciliation for this object. Status shows Synced=False ReasonSuspended.
	// +optional
	Suspend bool `json:"suspend,omitempty"`
}

// ChannelStatus defines the observed state of Channel.
type ChannelStatus struct {
	// Conditions represent the current state of the channel.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration reflects the generation last successfully synced.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// DesiredMQSC is a debug/GitOps aid: the DEFINE CHANNEL REPLACE line equivalent
	// to what the operator applies via mqweb. Not authoritative; do not use this
	// field to drive cluster apply or drift detection.
	// +optional
	DesiredMQSC string `json:"desiredMQSC,omitempty"`

	MQObjectStatusFields `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=chl
// +kubebuilder:printcolumn:name="Synced",type=string,JSONPath=`.status.conditions[?(@.type=="Synced")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Synced")].reason`
// +kubebuilder:printcolumn:name="Channel",type=string,JSONPath=`.spec.channelName`
// +kubebuilder:printcolumn:name="Desired MQSC",type=string,JSONPath=`.status.desiredMQSC`,priority=1
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Channel maintains an IBM MQ channel on a referenced queue manager.
type Channel struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ChannelSpec   `json:"spec,omitempty"`
	Status ChannelStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ChannelList contains a list of Channel.
type ChannelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Channel `json:"items"`
}

func init() {
	Register(&Channel{}, &ChannelList{})
}
