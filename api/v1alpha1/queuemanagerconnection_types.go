package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Condition types for Kurator resources.
const (
	ConditionReady  = "Ready"
	ConditionSynced = "Synced"
)

// Condition reasons shared across resources.
const (
	ReasonAvailable   = "Available"
	ReasonProgressing = "Progressing"
	ReasonError       = "Error"
	ReasonDeleting    = "Deleting"
)

// QueueManagerConnectionFinalizer ensures MQ cleanup completes before removal.
const QueueManagerConnectionFinalizer = "messaging.kurator.dev/connection"

// AllowInsecureTLSAnnotation opts in to tls.insecureSkipVerify on QueueManagerConnection (dev only).
const AllowInsecureTLSAnnotation = "messaging.kurator.dev/allow-insecure-tls"

// QueueFinalizer ensures the MQ queue is deleted before the CR is removed.
const QueueFinalizer = "messaging.kurator.dev/queue"

// QueueManagerConnectionSpec defines how to reach an IBM MQ queue manager.
type QueueManagerConnectionSpec struct {
	// QueueManager is the IBM MQ queue manager name (case-sensitive).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	QueueManager string `json:"queueManager"`

	// Endpoint is the mqweb base URL, e.g. https://ibm-mq.ibm-mq.svc:9443
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^https://`
	Endpoint string `json:"endpoint"`

	// RESTPrefix is the mqweb REST API path prefix. Defaults to /ibmmq/rest/v3.
	// +kubebuilder:validation:Pattern=`^/`
	// +optional
	RESTPrefix string `json:"restPrefix,omitempty"`

	// TLS configures HTTPS trust for mqweb.
	// +optional
	TLS *TLSConfig `json:"tls,omitempty"`

	// CredentialsSecretRef references a Secret with mqweb credentials.
	// Required keys: username, password (mqAdminPassword is also accepted for password).
	// +kubebuilder:validation:Required
	CredentialsSecretRef SecretReference `json:"credentialsSecretRef"`
}

// TLSConfig holds TLS options for mqweb.
type TLSConfig struct {
	// InsecureSkipVerify disables server certificate verification (dev only).
	// +optional
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`

	// CASecretRef references a Secret containing a CA bundle (key tls.crt or ca.crt).
	// +optional
	CASecretRef *SecretReference `json:"caSecretRef,omitempty"`
}

// SecretReference identifies a Secret in the same namespace as the CR.
type SecretReference struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

// QueueManagerConnectionStatus defines the observed state of QueueManagerConnection.
type QueueManagerConnectionStatus struct {
	// Conditions represent the current state of the connection.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration reflects the generation last reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=qmc
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// QueueManagerConnection describes how to reach an IBM MQ queue manager via mqweb.
type QueueManagerConnection struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   QueueManagerConnectionSpec   `json:"spec,omitempty"`
	Status QueueManagerConnectionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// QueueManagerConnectionList contains a list of QueueManagerConnection.
type QueueManagerConnectionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []QueueManagerConnection `json:"items"`
}

func init() {
	SchemeBuilder.Register(&QueueManagerConnection{}, &QueueManagerConnectionList{})
}
