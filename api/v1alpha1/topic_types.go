package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TopicAccessEnabled controls whether publish or subscribe is allowed on a topic (MQSC PUB/SUB).
// +kubebuilder:validation:Enum=enabled;disabled
type TopicAccessEnabled string

const (
	// TopicAccessEnabledEnabled allows publish or subscribe on the topic node.
	TopicAccessEnabledEnabled TopicAccessEnabled = "enabled"
	// TopicAccessEnabledDisabled disallows publish or subscribe on the topic node.
	TopicAccessEnabledDisabled TopicAccessEnabled = "disabled"
)

// TopicFinalizer ensures the MQ topic is deleted before the CR is removed.
const TopicFinalizer = "messaging.mkurator.dev/topic"

// TopicSpec defines an administrative topic object on a referenced queue manager.
// +kubebuilder:validation:XValidation:rule="!has(self.topicString) || self.topicString.size() == 0 || !has(self.attributes) || !self.attributes.exists(k, k.lowerAscii() == 'topstr' || k.lowerAscii() == 'topicstr')",message="topicString field and attributes.topstr (or topicstr) are mutually exclusive"
// +kubebuilder:validation:XValidation:rule="!has(self.description) || self.description.size() == 0 || !has(self.attributes) || !self.attributes.exists(k, k.lowerAscii() == 'descr')",message="description field and attributes.descr are mutually exclusive"
// +kubebuilder:validation:XValidation:rule="!has(self.publish) || !has(self.attributes) || !self.attributes.exists(k, k.lowerAscii() == 'pub')",message="publish field and attributes.pub are mutually exclusive"
// +kubebuilder:validation:XValidation:rule="!has(self.subscribe) || !has(self.attributes) || !self.attributes.exists(k, k.lowerAscii() == 'sub')",message="subscribe field and attributes.sub are mutually exclusive"
// +kubebuilder:validation:XValidation:rule="!has(self.defPersistence) || !has(self.attributes) || !self.attributes.exists(k, k.lowerAscii() == 'defpsist')",message="defPersistence field and attributes.defpsist are mutually exclusive"
// +kubebuilder:validation:XValidation:rule="!has(self.publishScope) || self.publishScope.size() == 0 || !has(self.attributes) || !self.attributes.exists(k, k.lowerAscii() == 'pubscope')",message="publishScope field and attributes.pubscope are mutually exclusive"
// +kubebuilder:validation:XValidation:rule="!has(self.subscribeScope) || self.subscribeScope.size() == 0 || !has(self.attributes) || !self.attributes.exists(k, k.lowerAscii() == 'subscope')",message="subscribeScope field and attributes.subscope are mutually exclusive"
type TopicSpec struct {
	// ConnectionRef names a QueueManagerConnection in the same namespace.
	// +kubebuilder:validation:Required
	ConnectionRef LocalObjectReference `json:"connectionRef"`

	// TopicName is the IBM MQ topic object name (e.g. RETAIL.ORDERS).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=48
	// +kubebuilder:validation:Pattern=`^[A-Z0-9./%&$#@]+$`
	// +kubebuilder:validation:XValidation:rule="self == self.trim()",message="name must not have leading or trailing whitespace"
	// +kubebuilder:validation:XValidation:rule="!self.startsWith('.') && !self.endsWith('.')",message="name must not start or end with '.'"
	// +kubebuilder:validation:XValidation:rule="!self.upperAscii().startsWith('SYSTEM.')",message="names with prefix SYSTEM. are reserved for queue manager objects"
	// +kubebuilder:validation:XValidation:rule="!self.upperAscii().startsWith('AMQ')",message="names with prefix AMQ are reserved for IBM MQ internal use"
	TopicName string `json:"topicName"`

	// Attributes map to MQSC parameters (lowercase keys in mqweb runCommandJSON).
	// Drift-checked vs define-only keys: docs/ATTRIBUTE_RECONCILIATION.md.
	// +optional
	Attributes map[string]string `json:"attributes,omitempty"`

	// TopicString is the IBM MQ topic string (MQSC TOPICSTR / attribute topstr).
	// Mutually exclusive with attributes.topstr and attributes.topicstr; typed field
	// takes precedence when folded into the attribute map for mqadmin.
	// +optional
	TopicString string `json:"topicString,omitempty"`

	// Description is the topic description (MQSC DESCR).
	// Mutually exclusive with attributes.descr; typed field takes precedence when folded
	// into the attribute map for mqadmin.
	// +optional
	Description string `json:"description,omitempty"`

	// Publish controls whether applications may publish to the topic (MQSC PUB).
	// Mutually exclusive with attributes.pub; typed field takes precedence when folded
	// into the attribute map for mqadmin.
	// +optional
	Publish TopicAccessEnabled `json:"publish,omitempty"`

	// Subscribe controls whether applications may subscribe to the topic (MQSC SUB).
	// Mutually exclusive with attributes.sub; typed field takes precedence when folded
	// into the attribute map for mqadmin.
	// +optional
	Subscribe TopicAccessEnabled `json:"subscribe,omitempty"`

	// DefPersistence is the default message persistence for new messages (MQSC DEFPSIST).
	// Mutually exclusive with attributes.defpsist; typed field takes precedence when folded
	// into the attribute map for mqadmin.
	// +optional
	DefPersistence QueueDefaultPersistence `json:"defPersistence,omitempty"`

	// PublishScope is the publish scope for the topic (MQSC PUBSCOPE).
	// Mutually exclusive with attributes.pubscope; typed field takes precedence when folded
	// into the attribute map for mqadmin.
	// +optional
	PublishScope string `json:"publishScope,omitempty"`

	// SubscribeScope is the subscribe scope for the topic (MQSC SUBSCOPE).
	// Mutually exclusive with attributes.subscope; typed field takes precedence when folded
	// into the attribute map for mqadmin.
	// +optional
	SubscribeScope string `json:"subscribeScope,omitempty"`

	// Suspend pauses MQ reconciliation for this object. Status shows Synced=False ReasonSuspended.
	// +optional
	Suspend bool `json:"suspend,omitempty"`

	WorkloadLifecyclePolicies `json:",inline"`
}

// TopicStatus defines the observed state of Topic.
type TopicStatus struct {
	// Conditions represent the current state of the topic.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration reflects the generation last successfully synced.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// DesiredMQSC is a debug/GitOps aid: the DEFINE TOPIC REPLACE line equivalent
	// to what the operator applies via mqweb. Not authoritative; do not use this
	// field to drive cluster apply or drift detection.
	// +optional
	DesiredMQSC string `json:"desiredMQSC,omitempty"`

	MQObjectStatusFields `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=tp
// +kubebuilder:printcolumn:name="Synced",type=string,JSONPath=`.status.conditions[?(@.type=="Synced")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Synced")].reason`
// +kubebuilder:printcolumn:name="Topic",type=string,JSONPath=`.spec.topicName`
// +kubebuilder:printcolumn:name="Desired MQSC",type=string,JSONPath=`.status.desiredMQSC`,priority=1
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Topic maintains an IBM MQ topic object on a referenced queue manager.
type Topic struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TopicSpec   `json:"spec,omitempty"`
	Status TopicStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TopicList contains a list of Topic.
type TopicList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Topic `json:"items"`
}

func init() {
	Register(&Topic{}, &TopicList{})
}
