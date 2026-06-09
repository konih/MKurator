package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AuthorityObjectType is the AUTHREC OBJTYPE.
// +kubebuilder:validation:Enum=QUEUE;CHANNEL;TOPIC;QMGR;NAMESPAC;PROCESS;NLIST
type AuthorityObjectType string

const (
	AuthorityObjectTypeQueue     AuthorityObjectType = "QUEUE"
	AuthorityObjectTypeChannel   AuthorityObjectType = "CHANNEL"
	AuthorityObjectTypeTopic     AuthorityObjectType = "TOPIC"
	AuthorityObjectTypeQMGR      AuthorityObjectType = "QMGR"
	AuthorityObjectTypeNamespace AuthorityObjectType = "NAMESPAC"
	AuthorityObjectTypeProcess   AuthorityObjectType = "PROCESS"
	AuthorityObjectTypeNList     AuthorityObjectType = "NLIST"
)

// AuthorityRecordFinalizer ensures OAM records are removed before the CR is deleted.
const AuthorityRecordFinalizer = "messaging.mkurator.dev/authorityrecord"

// AuthorityRecordSpec defines a SET AUTHREC rule on a referenced queue manager.
type AuthorityRecordSpec struct {
	// ConnectionRef names a QueueManagerConnection in the same namespace.
	// +kubebuilder:validation:Required
	ConnectionRef LocalObjectReference `json:"connectionRef"`

	// Profile maps to PROFILE('…') — queue, channel, or other object name.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Profile string `json:"profile"`

	// ObjectType maps to OBJTYPE(...).
	// +kubebuilder:validation:Required
	ObjectType AuthorityObjectType `json:"objectType"`

	// Principal maps to PRINCIPAL('…'). Mutually exclusive with Group.
	// +optional
	Principal string `json:"principal,omitempty"`

	// Group maps to GROUP('…'). Mutually exclusive with Principal.
	// +optional
	Group string `json:"group,omitempty"`

	// Authorities maps to AUTHADD(...) — e.g. GET, PUT, CONNECT.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:items:Pattern=`^[A-Za-z0-9+_]+$`
	// +listType=set
	Authorities []string `json:"authorities"`

	// Suspend pauses MQ reconciliation for this object. Status shows Synced=False ReasonSuspended.
	// +optional
	Suspend bool `json:"suspend,omitempty"`
}

// AuthorityRecordStatus defines the observed state of AuthorityRecord.
type AuthorityRecordStatus struct {
	// Conditions represent the current state of the authority record.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration reflects the generation last successfully synced.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// DesiredMQSC is a debug/GitOps aid: the SET AUTHREC AUTHADD(...) line equivalent
	// to what the operator applies via mqweb. Not authoritative; do not use this
	// field to drive cluster apply or drift detection.
	// +optional
	DesiredMQSC string `json:"desiredMQSC,omitempty"`

	MQObjectStatusFields `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=auth
// +kubebuilder:printcolumn:name="Synced",type=string,JSONPath=`.status.conditions[?(@.type=="Synced")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Synced")].reason`
// +kubebuilder:printcolumn:name="Profile",type=string,JSONPath=`.spec.profile`
// +kubebuilder:printcolumn:name="Desired MQSC",type=string,JSONPath=`.status.desiredMQSC`,priority=1
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// AuthorityRecord maintains IBM MQ OAM authority on a referenced queue manager.
type AuthorityRecord struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AuthorityRecordSpec   `json:"spec,omitempty"`
	Status AuthorityRecordStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AuthorityRecordList contains a list of AuthorityRecord.
type AuthorityRecordList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AuthorityRecord `json:"items"`
}

func init() {
	Register(&AuthorityRecord{}, &AuthorityRecordList{})
}
