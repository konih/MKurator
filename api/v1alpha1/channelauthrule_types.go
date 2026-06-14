package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ChannelAuthRuleType is the CHLAUTH rule TYPE.
// +kubebuilder:validation:Enum=ADDRESSMAP;BLOCKUSER;USERMAP;SSLPEERMAP;QMGRMAP;BLOCKADDR
type ChannelAuthRuleType string

const (
	ChannelAuthRuleTypeAddressMap ChannelAuthRuleType = "ADDRESSMAP"
	ChannelAuthRuleTypeBlockUser  ChannelAuthRuleType = "BLOCKUSER"
	ChannelAuthRuleTypeUserMap    ChannelAuthRuleType = "USERMAP"
	ChannelAuthRuleTypeSSLPeerMap ChannelAuthRuleType = "SSLPEERMAP"
	ChannelAuthRuleTypeQMGRMap    ChannelAuthRuleType = "QMGRMAP"
	ChannelAuthRuleTypeBlockAddr  ChannelAuthRuleType = "BLOCKADDR"
)

// ChannelAuthUserSource is USERSRC for ADDRESSMAP and USERMAP rules.
// +kubebuilder:validation:Enum=CHANNEL;NOACCESS;MAP
type ChannelAuthUserSource string

const (
	ChannelAuthUserSourceChannel  ChannelAuthUserSource = "CHANNEL"
	ChannelAuthUserSourceNoAccess ChannelAuthUserSource = "NOACCESS"
	ChannelAuthUserSourceMap      ChannelAuthUserSource = "MAP"
)

// ChannelAuthCheckClient is CHCKCLNT for ADDRESSMAP rules.
// +kubebuilder:validation:Enum=REQUIRED;ASQMGR;REQDADM;ASCHL;OPTIONAL
type ChannelAuthCheckClient string

const (
	ChannelAuthCheckClientRequired ChannelAuthCheckClient = "REQUIRED"
	ChannelAuthCheckClientAsQMGR   ChannelAuthCheckClient = "ASQMGR"
	ChannelAuthCheckClientReqdAdm  ChannelAuthCheckClient = "REQDADM"
	ChannelAuthCheckClientAsCHL    ChannelAuthCheckClient = "ASCHL"
	ChannelAuthCheckClientOptional ChannelAuthCheckClient = "OPTIONAL"
)

// ChannelAuthRuleFinalizer ensures CHLAUTH is removed before the CR is deleted.
const ChannelAuthRuleFinalizer = "messaging.mkurator.dev/channelauthrule"

// ChannelAuthRuleSpec defines a SET CHLAUTH rule on a referenced queue manager.
// +kubebuilder:validation:XValidation:rule="self.ruleType != 'ADDRESSMAP' || (has(self.address) && size(self.address) > 0)",message="address is required for ADDRESSMAP rules"
// +kubebuilder:validation:XValidation:rule="self.ruleType != 'BLOCKADDR' || (has(self.address) && size(self.address) > 0)",message="address is required for BLOCKADDR rules"
// +kubebuilder:validation:XValidation:rule="self.ruleType != 'BLOCKUSER' || (has(self.userList) && size(self.userList) > 0)",message="userList is required for BLOCKUSER rules"
// +kubebuilder:validation:XValidation:rule="self.ruleType != 'USERMAP' || (has(self.clientUser) && size(self.clientUser) > 0)",message="clientUser is required for USERMAP rules"
// +kubebuilder:validation:XValidation:rule="self.ruleType != 'SSLPEERMAP' || (has(self.sslPeerName) && size(self.sslPeerName) > 0)",message="sslPeerName is required for SSLPEERMAP rules"
// +kubebuilder:validation:XValidation:rule="self.ruleType != 'QMGRMAP' || (has(self.remoteQueueManager) && size(self.remoteQueueManager) > 0)",message="remoteQueueManager is required for QMGRMAP rules"
// +kubebuilder:validation:XValidation:rule="(self.ruleType != 'USERMAP' && self.ruleType != 'SSLPEERMAP' && self.ruleType != 'QMGRMAP') || self.userSource != 'MAP' || (has(self.mcaUser) && size(self.mcaUser) > 0)",message="mcaUser is required when userSource is MAP"
type ChannelAuthRuleSpec struct {
	// ConnectionRef names a QueueManagerConnection in the same namespace.
	// +kubebuilder:validation:Required
	ConnectionRef LocalObjectReference `json:"connectionRef"`

	// ChannelName is the IBM MQ channel name in SET CHLAUTH('…').
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=48
	// +kubebuilder:validation:Pattern=`^[A-Z0-9./%&$#@*]+$`
	// +kubebuilder:validation:XValidation:rule="self == self.trim()",message="name must not have leading or trailing whitespace"
	// +kubebuilder:validation:XValidation:rule="!self.startsWith('.') && !self.endsWith('.')",message="name must not start or end with '.'"
	// +kubebuilder:validation:XValidation:rule="!self.upperAscii().startsWith('SYSTEM.')",message="names with prefix SYSTEM. are reserved for queue manager objects"
	// +kubebuilder:validation:XValidation:rule="!self.upperAscii().startsWith('AMQ')",message="names with prefix AMQ are reserved for IBM MQ internal use"
	ChannelName string `json:"channelName"`

	// RuleType maps to CHLAUTH TYPE(...).
	// +kubebuilder:validation:Required
	RuleType ChannelAuthRuleType `json:"ruleType"`

	// Address maps to ADDRESS(...) for ADDRESSMAP and BLOCKADDR rules.
	// +optional
	Address string `json:"address,omitempty"`

	// UserList maps to USERLIST(...) for BLOCKUSER rules.
	// +optional
	UserList string `json:"userList,omitempty"`

	// ClientUser maps to CLNTUSER(...) for USERMAP rules.
	// +optional
	ClientUser string `json:"clientUser,omitempty"`

	// SslPeerName maps to SSLPEER(...) for SSLPEERMAP rules (TLS certificate DN pattern).
	// +optional
	SslPeerName string `json:"sslPeerName,omitempty"`

	// RemoteQueueManager maps to QMNAME(...) for QMGRMAP rules (remote partner queue manager name).
	// +optional
	RemoteQueueManager string `json:"remoteQueueManager,omitempty"`

	// McaUser maps to MCAUSER(...) for USERMAP (USERSRC MAP), SSLPEERMAP (USERSRC MAP),
	// QMGRMAP (USERSRC MAP), and ADDRESSMAP (USERSRC MAP) rules.
	// +optional
	McaUser string `json:"mcaUser,omitempty"`

	// UserSource maps to USERSRC(...) for ADDRESSMAP, USERMAP, SSLPEERMAP, and QMGRMAP rules.
	// +optional
	UserSource ChannelAuthUserSource `json:"userSource,omitempty"`

	// CheckClient maps to CHCKCLNT(...) for ADDRESSMAP rules.
	// +optional
	CheckClient ChannelAuthCheckClient `json:"checkClient,omitempty"`

	// Description maps to DESCR(...).
	// +optional
	Description string `json:"description,omitempty"`

	// Suspend pauses MQ reconciliation for this object. Status shows Synced=False ReasonSuspended.
	// +optional
	Suspend bool `json:"suspend,omitempty"`

	WorkloadLifecyclePolicies `json:",inline"`
}

// ChannelAuthRuleStatus defines the observed state of ChannelAuthRule.
type ChannelAuthRuleStatus struct {
	// Conditions represent the current state of the rule.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration reflects the generation last successfully synced.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// DesiredMQSC is a debug/GitOps aid: the SET CHLAUTH ACTION(REPLACE) line
	// equivalent to what the operator applies via mqweb. Not authoritative; do not
	// use this field to drive cluster apply or drift detection.
	// +optional
	DesiredMQSC string `json:"desiredMQSC,omitempty"`

	MQObjectStatusFields `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=car
// +kubebuilder:printcolumn:name="Synced",type=string,JSONPath=`.status.conditions[?(@.type=="Synced")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Synced")].reason`
// +kubebuilder:printcolumn:name="Channel",type=string,JSONPath=`.spec.channelName`
// +kubebuilder:printcolumn:name="Desired MQSC",type=string,JSONPath=`.status.desiredMQSC`,priority=1
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ChannelAuthRule maintains an IBM MQ CHLAUTH rule on a referenced queue manager.
type ChannelAuthRule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ChannelAuthRuleSpec   `json:"spec,omitempty"`
	Status ChannelAuthRuleStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ChannelAuthRuleList contains a list of ChannelAuthRule.
type ChannelAuthRuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ChannelAuthRule `json:"items"`
}

func init() {
	Register(&ChannelAuthRule{}, &ChannelAuthRuleList{})
}
