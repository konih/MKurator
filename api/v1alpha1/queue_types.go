package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// QueueType is the IBM MQ queue object type to manage.
// +kubebuilder:validation:Enum=local;alias;remote
type QueueType string

const (
	QueueTypeLocal  QueueType = "local"
	QueueTypeAlias  QueueType = "alias"
	QueueTypeRemote QueueType = "remote"
)

// QueueDefaultPersistence is the default message persistence for new messages (MQSC DEFPSIST).
// +kubebuilder:validation:Enum=yes;no
type QueueDefaultPersistence string

const (
	QueueDefaultPersistenceYes QueueDefaultPersistence = "yes"
	QueueDefaultPersistenceNo  QueueDefaultPersistence = "no"
)

// QueueSpec defines a queue to maintain on a referenced queue manager.
// +kubebuilder:validation:XValidation:rule="self.type != 'alias' || (has(self.attributes) && (('targq' in self.attributes && size(self.attributes['targq']) > 0) || ('target' in self.attributes && size(self.attributes['target']) > 0)))",message="alias queues require attribute targq (or target)"
// +kubebuilder:validation:XValidation:rule="self.type != 'remote' || (has(self.attributes) && (('xmitq' in self.attributes && size(self.attributes['xmitq']) > 0) || ('transmissionqueue' in self.attributes && size(self.attributes['transmissionqueue']) > 0)))",message="remote queues require attribute xmitq (or transmissionqueue)"
// +kubebuilder:validation:XValidation:rule="self.type != 'remote' || (has(self.attributes) && (('rqmname' in self.attributes && size(self.attributes['rqmname']) > 0) || ('remotemanager' in self.attributes && size(self.attributes['remotemanager']) > 0)))",message="remote queues require attribute rqmname (or remotemanager)"
// +kubebuilder:validation:XValidation:rule="!has(self.maxDepth) || !has(self.attributes) || !self.attributes.exists(k, k.lowerAscii() == 'maxdepth')",message="maxDepth field and attributes.maxdepth are mutually exclusive"
// +kubebuilder:validation:XValidation:rule="!has(self.description) || self.description.size() == 0 || !has(self.attributes) || !self.attributes.exists(k, k.lowerAscii() == 'descr')",message="description field and attributes.descr are mutually exclusive"
// +kubebuilder:validation:XValidation:rule="!has(self.defPersistence) || !has(self.attributes) || !self.attributes.exists(k, k.lowerAscii() == 'defpsist')",message="defPersistence field and attributes.defpsist are mutually exclusive"
type QueueSpec struct {
	// ConnectionRef names a QueueManagerConnection in the same namespace.
	// +kubebuilder:validation:Required
	ConnectionRef LocalObjectReference `json:"connectionRef"`

	// QueueName is the IBM MQ object name.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=48
	// +kubebuilder:validation:Pattern=`^[A-Z0-9./%&$#@]+$`
	// +kubebuilder:validation:XValidation:rule="self == self.trim()",message="name must not have leading or trailing whitespace"
	// +kubebuilder:validation:XValidation:rule="!self.startsWith('.') && !self.endsWith('.')",message="name must not start or end with '.'"
	// +kubebuilder:validation:XValidation:rule="!self.upperAscii().startsWith('SYSTEM.')",message="names with prefix SYSTEM. are reserved for queue manager objects"
	// +kubebuilder:validation:XValidation:rule="!self.upperAscii().startsWith('AMQ')",message="names with prefix AMQ are reserved for IBM MQ internal use"
	QueueName string `json:"queueName"`

	// Type is the queue kind to define: local (QLOCAL), alias (QALIAS), or remote (QREMOTE).
	// +kubebuilder:default=local
	// +optional
	Type QueueType `json:"type,omitempty"`

	// Attributes map to MQSC parameters (lowercase keys in mqweb runCommandJSON).
	// Drift-checked vs define-only keys: docs/ATTRIBUTE_RECONCILIATION.md.
	// +optional
	Attributes map[string]string `json:"attributes,omitempty"`

	// MaxDepth is the maximum number of messages the queue can hold (MQSC MAXDEPTH).
	// Mutually exclusive with attributes.maxdepth; typed field takes precedence when folded
	// into the attribute map for mqadmin.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=999999999
	// +optional
	MaxDepth *int32 `json:"maxDepth,omitempty"`

	// Description is the queue description (MQSC DESCR).
	// Mutually exclusive with attributes.descr; typed field takes precedence when folded
	// into the attribute map for mqadmin.
	// +optional
	Description string `json:"description,omitempty"`

	// DefPersistence is the default message persistence for new messages (MQSC DEFPSIST).
	// Mutually exclusive with attributes.defpsist; typed field takes precedence when folded
	// into the attribute map for mqadmin.
	// +optional
	DefPersistence QueueDefaultPersistence `json:"defPersistence,omitempty"`

	// Suspend pauses MQ reconciliation for this object. Status shows Synced=False ReasonSuspended.
	// +optional
	Suspend bool `json:"suspend,omitempty"`

	WorkloadLifecyclePolicies `json:",inline"`
}

// LocalObjectReference is a namespaced object reference.
type LocalObjectReference struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

// QueueStatus defines the observed state of Queue.
type QueueStatus struct {
	// Conditions represent the current state of the queue.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration reflects the generation last successfully synced.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// DesiredMQSC is a debug/GitOps aid: the DEFINE QLOCAL|QALIAS|QREMOTE REPLACE
	// line equivalent to what the operator applies via mqweb. Not authoritative;
	// do not use this field to drive cluster apply or drift detection.
	// +optional
	DesiredMQSC string `json:"desiredMQSC,omitempty"`

	MQObjectStatusFields `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=mq
// +kubebuilder:printcolumn:name="Synced",type=string,JSONPath=`.status.conditions[?(@.type=="Synced")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Synced")].reason`
// +kubebuilder:printcolumn:name="Queue",type=string,JSONPath=`.spec.queueName`
// +kubebuilder:printcolumn:name="Desired MQSC",type=string,JSONPath=`.status.desiredMQSC`,priority=1
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Queue maintains an IBM MQ queue on a referenced queue manager.
type Queue struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   QueueSpec   `json:"spec,omitempty"`
	Status QueueStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// QueueList contains a list of Queue.
type QueueList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Queue `json:"items"`
}

func init() {
	Register(&Queue{}, &QueueList{})
}
