package webhookv1alpha1

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	messagingv1alpha1 "github.com/konih/kurator/api/v1alpha1"
)

func webhookTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := messagingv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add scheme: %v", err)
	}
	return scheme
}

func sampleWebhookConn(ns string) *messagingv1alpha1.QueueManagerConnection {
	return &messagingv1alpha1.QueueManagerConnection{
		ObjectMeta: metav1.ObjectMeta{Name: "qm1", Namespace: ns},
		Spec: messagingv1alpha1.QueueManagerConnectionSpec{
			QueueManager:         "QM1",
			Endpoint:             "https://mq.example:9443",
			CredentialsSecretRef: messagingv1alpha1.SecretReference{Name: "creds"},
		},
	}
}

func sampleWebhookChannel(ns, name, channelName string) *messagingv1alpha1.Channel {
	return &messagingv1alpha1.Channel{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: messagingv1alpha1.ChannelSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			ChannelName:   channelName,
		},
	}
}

func TestTopicWebhookValidateCreate(t *testing.T) {
	scheme := webhookTestScheme(t)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sampleWebhookConn("ns")).Build()
	v := &topicCustomValidator{Client: cl}

	topic := &messagingv1alpha1.Topic{
		ObjectMeta: metav1.ObjectMeta{Name: "t1", Namespace: "ns"},
		Spec: messagingv1alpha1.TopicSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			TopicName:     "RETAIL.ORDERS",
		},
	}
	if _, err := v.ValidateCreate(context.Background(), topic); err != nil {
		t.Fatalf("ValidateCreate: %v", err)
	}
}

func TestTopicWebhookValidateUpdate(t *testing.T) {
	scheme := webhookTestScheme(t)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sampleWebhookConn("ns")).Build()
	v := &topicCustomValidator{Client: cl}

	topic := &messagingv1alpha1.Topic{
		ObjectMeta: metav1.ObjectMeta{Name: "t1", Namespace: "ns"},
		Spec: messagingv1alpha1.TopicSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			TopicName:     "RETAIL.ORDERS",
		},
	}
	if _, err := v.ValidateUpdate(context.Background(), topic, topic); err != nil {
		t.Fatalf("ValidateUpdate: %v", err)
	}
}

func TestTopicWebhookValidateDelete(t *testing.T) {
	v := &topicCustomValidator{}
	if _, err := v.ValidateDelete(context.Background(), nil); err != nil {
		t.Fatalf("ValidateDelete: %v", err)
	}
}

func TestChannelWebhookValidateCreate(t *testing.T) {
	scheme := webhookTestScheme(t)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sampleWebhookConn("ns")).Build()
	v := &channelCustomValidator{Client: cl}

	channel := &messagingv1alpha1.Channel{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "ns"},
		Spec: messagingv1alpha1.ChannelSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			ChannelName:   "ORDERS.APP",
		},
	}
	if _, err := v.ValidateCreate(context.Background(), channel); err != nil {
		t.Fatalf("ValidateCreate: %v", err)
	}
}

func TestChannelWebhookValidateUpdate(t *testing.T) {
	scheme := webhookTestScheme(t)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sampleWebhookConn("ns")).Build()
	v := &channelCustomValidator{Client: cl}

	channel := &messagingv1alpha1.Channel{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "ns"},
		Spec: messagingv1alpha1.ChannelSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			ChannelName:   "ORDERS.APP",
		},
	}
	if _, err := v.ValidateUpdate(context.Background(), channel, channel); err != nil {
		t.Fatalf("ValidateUpdate: %v", err)
	}
}

func TestChannelWebhookValidateDelete(t *testing.T) {
	v := &channelCustomValidator{}
	if _, err := v.ValidateDelete(context.Background(), nil); err != nil {
		t.Fatalf("ValidateDelete: %v", err)
	}
}

func TestQueueWebhookValidateCreate(t *testing.T) {
	scheme := webhookTestScheme(t)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sampleWebhookConn("ns")).Build()
	v := &queueCustomValidator{Client: cl}

	queue := &messagingv1alpha1.Queue{
		ObjectMeta: metav1.ObjectMeta{Name: "q1", Namespace: "ns"},
		Spec: messagingv1alpha1.QueueSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			QueueName:     "APP.ORDERS",
		},
	}
	if _, err := v.ValidateCreate(context.Background(), queue); err != nil {
		t.Fatalf("ValidateCreate: %v", err)
	}
}

func TestQueueWebhookValidateUpdate(t *testing.T) {
	scheme := webhookTestScheme(t)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sampleWebhookConn("ns")).Build()
	v := &queueCustomValidator{Client: cl}

	queue := &messagingv1alpha1.Queue{
		ObjectMeta: metav1.ObjectMeta{Name: "q1", Namespace: "ns"},
		Spec: messagingv1alpha1.QueueSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			QueueName:     "APP.ORDERS",
		},
	}
	if _, err := v.ValidateUpdate(context.Background(), queue, queue); err != nil {
		t.Fatalf("ValidateUpdate: %v", err)
	}
}

func TestQueueWebhookValidateDelete(t *testing.T) {
	v := &queueCustomValidator{}
	if _, err := v.ValidateDelete(context.Background(), nil); err != nil {
		t.Fatalf("ValidateDelete: %v", err)
	}
}

func TestQueueManagerConnectionWebhookValidateCreate(t *testing.T) {
	scheme := webhookTestScheme(t)
	_ = corev1.AddToScheme(scheme)
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "creds", Namespace: "ns"}}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()
	v := &queueManagerConnectionCustomValidator{Client: cl}

	conn := sampleWebhookConn("ns")
	if _, err := v.ValidateCreate(context.Background(), conn); err != nil {
		t.Fatalf("ValidateCreate: %v", err)
	}
}

func TestQueueManagerConnectionWebhookValidateUpdate(t *testing.T) {
	scheme := webhookTestScheme(t)
	_ = corev1.AddToScheme(scheme)
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "creds", Namespace: "ns"}}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()
	v := &queueManagerConnectionCustomValidator{Client: cl}

	conn := sampleWebhookConn("ns")
	if _, err := v.ValidateUpdate(context.Background(), conn, conn); err != nil {
		t.Fatalf("ValidateUpdate: %v", err)
	}
}

func TestQueueManagerConnectionWebhookValidateInvalidSpec(t *testing.T) {
	scheme := webhookTestScheme(t)
	_ = corev1.AddToScheme(scheme)
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	v := &queueManagerConnectionCustomValidator{Client: cl}

	conn := &messagingv1alpha1.QueueManagerConnection{
		ObjectMeta: metav1.ObjectMeta{Name: "bad", Namespace: "ns"},
		Spec:       messagingv1alpha1.QueueManagerConnectionSpec{},
	}
	if _, err := v.validate(context.Background(), conn); err == nil {
		t.Fatal("expected validation error for empty spec")
	}
}

func TestQueueManagerConnectionWebhookValidateInsecureTLS(t *testing.T) {
	scheme := webhookTestScheme(t)
	_ = corev1.AddToScheme(scheme)
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "creds", Namespace: "ns"}}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()
	v := &queueManagerConnectionCustomValidator{Client: cl}

	conn := sampleWebhookConn("ns")
	conn.Spec.TLS = &messagingv1alpha1.TLSConfig{InsecureSkipVerify: true}

	if _, err := v.ValidateCreate(context.Background(), conn); err == nil {
		t.Fatal("expected deny without allow-insecure-tls annotation")
	}

	conn.Annotations = map[string]string{
		messagingv1alpha1.AllowInsecureTLSAnnotation: "true",
	}
	if _, err := v.ValidateCreate(context.Background(), conn); err != nil {
		t.Fatalf("ValidateCreate with opt-in annotation: %v", err)
	}
}

func TestQueueManagerConnectionWebhookValidateDelete(t *testing.T) {
	scheme := webhookTestScheme(t)
	conn := sampleWebhookConn("ns")
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(conn).Build()
	v := &queueManagerConnectionCustomValidator{Client: cl}

	warnings, err := v.ValidateDelete(context.Background(), conn)
	if err != nil {
		t.Fatalf("ValidateDelete: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
}

func TestChannelAuthRuleWebhookValidateCreate(t *testing.T) {
	scheme := webhookTestScheme(t)
	cl := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(sampleWebhookConn("ns"), sampleWebhookChannel("ns", "orders-app", "ORDERS.APP")).
		Build()
	v := &channelAuthRuleCustomValidator{Client: cl}

	rule := &messagingv1alpha1.ChannelAuthRule{
		ObjectMeta: metav1.ObjectMeta{Name: "car1", Namespace: "ns"},
		Spec: messagingv1alpha1.ChannelAuthRuleSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			ChannelName:   "ORDERS.APP",
			RuleType:      messagingv1alpha1.ChannelAuthRuleTypeAddressMap,
			Address:       "*",
		},
	}
	if _, err := v.ValidateCreate(context.Background(), rule); err != nil {
		t.Fatalf("ValidateCreate: %v", err)
	}
}

func TestAuthorityRecordWebhookValidateCreate(t *testing.T) {
	scheme := webhookTestScheme(t)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sampleWebhookConn("ns")).Build()
	v := &authorityRecordCustomValidator{Client: cl}

	auth := &messagingv1alpha1.AuthorityRecord{
		ObjectMeta: metav1.ObjectMeta{Name: "auth1", Namespace: "ns"},
		Spec: messagingv1alpha1.AuthorityRecordSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			Profile:       "APP.ORDERS",
			ObjectType:    messagingv1alpha1.AuthorityObjectTypeQueue,
			Principal:     "app",
			Authorities:   []string{"GET", "PUT"},
		},
	}
	if _, err := v.ValidateCreate(context.Background(), auth); err != nil {
		t.Fatalf("ValidateCreate: %v", err)
	}
}

func TestChannelAuthRuleWebhookValidateCreateMissingChannel(t *testing.T) {
	scheme := webhookTestScheme(t)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sampleWebhookConn("ns")).Build()
	v := &channelAuthRuleCustomValidator{Client: cl}

	rule := &messagingv1alpha1.ChannelAuthRule{
		ObjectMeta: metav1.ObjectMeta{Name: "car1", Namespace: "ns"},
		Spec: messagingv1alpha1.ChannelAuthRuleSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			ChannelName:   "ORDERS.APP",
			RuleType:      messagingv1alpha1.ChannelAuthRuleTypeAddressMap,
			Address:       "*",
		},
	}
	if _, err := v.ValidateCreate(context.Background(), rule); err == nil {
		t.Fatal("expected ValidateCreate error when managed Channel missing")
	}
}

func TestChannelAuthRuleWebhookValidateUpdateDelete(t *testing.T) {
	scheme := webhookTestScheme(t)
	cl := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(sampleWebhookConn("ns"), sampleWebhookChannel("ns", "orders-app", "ORDERS.APP")).
		Build()
	v := &channelAuthRuleCustomValidator{Client: cl}
	rule := &messagingv1alpha1.ChannelAuthRule{
		ObjectMeta: metav1.ObjectMeta{Name: "car1", Namespace: "ns"},
		Spec: messagingv1alpha1.ChannelAuthRuleSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			ChannelName:   "ORDERS.APP",
			RuleType:      messagingv1alpha1.ChannelAuthRuleTypeAddressMap,
			Address:       "*",
		},
	}
	if _, err := v.ValidateUpdate(context.Background(), rule, rule); err != nil {
		t.Fatalf("ValidateUpdate: %v", err)
	}
	if _, err := v.ValidateDelete(context.Background(), rule); err != nil {
		t.Fatalf("ValidateDelete: %v", err)
	}
}

func TestChannelAuthRuleWebhookValidateUpdateDuringDeleteSkipsSpec(t *testing.T) {
	scheme := webhookTestScheme(t)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sampleWebhookConn("ns")).Build()
	v := &channelAuthRuleCustomValidator{Client: cl}
	now := metav1.Now()
	rule := &messagingv1alpha1.ChannelAuthRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "car1",
			Namespace:         "ns",
			DeletionTimestamp: &now,
			Finalizers:        []string{messagingv1alpha1.ChannelAuthRuleFinalizer},
		},
		Spec: messagingv1alpha1.ChannelAuthRuleSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			ChannelName:   "ORDERS.APP",
			RuleType:      messagingv1alpha1.ChannelAuthRuleTypeAddressMap,
			Address:       "*",
		},
	}
	if _, err := v.ValidateUpdate(context.Background(), rule, rule); err != nil {
		t.Fatalf("ValidateUpdate during delete: %v", err)
	}
}

func TestAuthorityRecordWebhookValidateUpdateDuringDeleteSkipsSpec(t *testing.T) {
	scheme := webhookTestScheme(t)
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	v := &authorityRecordCustomValidator{Client: cl}
	now := metav1.Now()
	auth := &messagingv1alpha1.AuthorityRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "auth1",
			Namespace:         "ns",
			DeletionTimestamp: &now,
			Finalizers:        []string{messagingv1alpha1.AuthorityRecordFinalizer},
		},
		Spec: messagingv1alpha1.AuthorityRecordSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			Profile:       "APP.ORDERS",
			ObjectType:    messagingv1alpha1.AuthorityObjectTypeQueue,
			Principal:     "app",
			Authorities:   []string{"GET", "PUT"},
		},
	}
	if _, err := v.ValidateUpdate(context.Background(), auth, auth); err != nil {
		t.Fatalf("ValidateUpdate during delete: %v", err)
	}
}

func TestAuthorityRecordWebhookValidateUpdateDelete(t *testing.T) {
	scheme := webhookTestScheme(t)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sampleWebhookConn("ns")).Build()
	v := &authorityRecordCustomValidator{Client: cl}
	auth := &messagingv1alpha1.AuthorityRecord{
		ObjectMeta: metav1.ObjectMeta{Name: "auth1", Namespace: "ns"},
		Spec: messagingv1alpha1.AuthorityRecordSpec{
			ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
			Profile:       "APP.ORDERS",
			ObjectType:    messagingv1alpha1.AuthorityObjectTypeQueue,
			Principal:     "app",
			Authorities:   []string{"GET", "PUT"},
		},
	}
	if _, err := v.ValidateUpdate(context.Background(), auth, auth); err != nil {
		t.Fatalf("ValidateUpdate: %v", err)
	}
	if _, err := v.ValidateDelete(context.Background(), auth); err != nil {
		t.Fatalf("ValidateDelete: %v", err)
	}
}
