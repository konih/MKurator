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

// ChannelTransportType is the channel transport protocol (MQSC TRPTYPE).
// +kubebuilder:validation:Enum=tcp;lu62
type ChannelTransportType string

const (
	// ChannelTransportTypeTCP is TCP/IP transport (typical for SVRCONN).
	ChannelTransportTypeTCP ChannelTransportType = "tcp"
	// ChannelTransportTypeLU62 is SNA LU6.2 transport.
	ChannelTransportTypeLU62 ChannelTransportType = "lu62"
)

// ChannelFinalizer ensures the MQ channel is deleted before the CR is removed.
const ChannelFinalizer = "messaging.mkurator.dev/channel"

// ChannelSpec defines a channel to maintain on a referenced queue manager.
// +kubebuilder:validation:XValidation:rule="!has(self.description) || self.description.size() == 0 || !has(self.attributes) || !self.attributes.exists(k, k.lowerAscii() == 'descr')",message="description field and attributes.descr are mutually exclusive"
// +kubebuilder:validation:XValidation:rule="!has(self.maxMsgLength) || !has(self.attributes) || !self.attributes.exists(k, k.lowerAscii() == 'maxmsgl')",message="maxMsgLength field and attributes.maxmsgl are mutually exclusive"
// +kubebuilder:validation:XValidation:rule="!has(self.transportType) || !has(self.attributes) || !self.attributes.exists(k, k.lowerAscii() == 'trptype')",message="transportType field and attributes.trptype are mutually exclusive"
type ChannelSpec struct {
	// ConnectionRef names a QueueManagerConnection in the same namespace.
	// +kubebuilder:validation:Required
	ConnectionRef LocalObjectReference `json:"connectionRef"`

	// ChannelName is the IBM MQ channel name (e.g. ORDERS.APP).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=48
	// +kubebuilder:validation:Pattern=`^[A-Z0-9./%&$#@]+$`
	// +kubebuilder:validation:XValidation:rule="self == self.trim()",message="name must not have leading or trailing whitespace"
	// +kubebuilder:validation:XValidation:rule="!self.startsWith('.') && !self.endsWith('.')",message="name must not start or end with '.'"
	// +kubebuilder:validation:XValidation:rule="!self.upperAscii().startsWith('SYSTEM.')",message="names with prefix SYSTEM. are reserved for queue manager objects"
	// +kubebuilder:validation:XValidation:rule="!self.upperAscii().startsWith('AMQ')",message="names with prefix AMQ are reserved for IBM MQ internal use"
	ChannelName string `json:"channelName"`

	// Type is the channel kind to define. Only svrconn is reconciled in v1alpha1.
	// +kubebuilder:default=svrconn
	// +optional
	Type ChannelType `json:"type,omitempty"`

	// Attributes map to MQSC parameters (lowercase keys in mqweb runCommandJSON).
	// Drift-checked vs define-only keys: docs/ATTRIBUTE_RECONCILIATION.md.
	// +optional
	Attributes map[string]string `json:"attributes,omitempty"`

	// Description is the channel description (MQSC DESCR).
	// Mutually exclusive with attributes.descr; typed field takes precedence when folded
	// into the attribute map for mqadmin.
	// +optional
	Description string `json:"description,omitempty"`

	// MaxMsgLength is the maximum message length in bytes (MQSC MAXMSGL).
	// Mutually exclusive with attributes.maxmsgl; typed field takes precedence when folded
	// into the attribute map for mqadmin.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=999999999
	// +optional
	MaxMsgLength *int32 `json:"maxMsgLength,omitempty"`

	// TransportType is the channel transport protocol (MQSC TRPTYPE).
	// Mutually exclusive with attributes.trptype; typed field takes precedence when folded
	// into the attribute map for mqadmin.
	// +optional
	TransportType ChannelTransportType `json:"transportType,omitempty"`

	// Suspend pauses MQ reconciliation for this object. Status shows Synced=False ReasonSuspended.
	// +optional
	Suspend bool `json:"suspend,omitempty"`

	WorkloadLifecyclePolicies `json:",inline"`
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
