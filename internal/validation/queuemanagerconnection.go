package validation

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"

	messagingv1alpha1 "github.com/konih/mkurator/api/v1alpha1"
)

const insecureTLSWithoutOptInMsg = "tls.insecureSkipVerify requires annotation " +
	messagingv1alpha1.AllowInsecureTLSAnnotation + `="true" (dev/local only; do not use in production)`

const credentialsUsernameDefaultWarningFmt = `credentials Secret %q has no username key ` +
	`(expected one of username, user, or mqAdminUser); mqweb login will default to "admin" — ` +
	`set an explicit username for production`

// ValidateQueueManagerConnectionSpec runs stateful admission validation for QueueManagerConnection.
func ValidateQueueManagerConnectionSpec(
	ctx context.Context,
	reader client.Reader,
	namespace string,
	annotations map[string]string,
	spec *messagingv1alpha1.QueueManagerConnectionSpec,
) ([]string, field.ErrorList) {
	var (
		warnings []string
		errs     field.ErrorList
	)

	if spec.CredentialsSecretRef.Name != "" {
		secretPath := field.NewPath("spec").Child("credentialsSecretRef").Child("name")
		secretErrs, credSecret := getSecretOrErrors(ctx, reader, namespace, spec.CredentialsSecretRef.Name, secretPath)
		errs = append(errs, secretErrs...)
		if credSecret != nil {
			warnings = append(warnings, credentialsSecretUsernameWarnings(credSecret)...)
		}
	}

	if spec.TLS != nil {
		if spec.TLS.InsecureSkipVerify && !allowInsecureTLS(annotations) {
			errs = append(errs, field.Invalid(field.NewPath("spec").Child("tls").Child("insecureSkipVerify"), true,
				insecureTLSWithoutOptInMsg))
		}
		if spec.TLS.CASecretRef != nil && spec.TLS.CASecretRef.Name != "" {
			errs = append(errs, validateSecretExists(ctx, reader, namespace,
				spec.TLS.CASecretRef.Name,
				field.NewPath("spec").Child("tls").Child("caSecretRef").Child("name"))...)
		}
	}

	return warnings, errs
}

func credentialsSecretUsernameWarnings(secret *corev1.Secret) []string {
	if credentialsSecretHasUsername(secret.Data) {
		return nil
	}
	return []string{fmt.Sprintf(credentialsUsernameDefaultWarningFmt, secret.Name)}
}

func credentialsSecretHasUsername(data map[string][]byte) bool {
	for _, key := range []string{"username", "user", "mqAdminUser"} {
		if v, ok := data[key]; ok && len(v) > 0 {
			return true
		}
	}
	return false
}

func allowInsecureTLS(annotations map[string]string) bool {
	if annotations == nil {
		return false
	}
	allowed, err := strconv.ParseBool(annotations[messagingv1alpha1.AllowInsecureTLSAnnotation])
	return err == nil && allowed
}

// ValidateQueueManagerConnectionDelete denies delete when Queue, Topic, Channel,
// ChannelAuthRule, or AuthorityRecord CRs in the same namespace reference this connection
// via spec.connectionRef.name.
func ValidateQueueManagerConnectionDelete(
	ctx context.Context,
	reader client.Reader,
	conn *messagingv1alpha1.QueueManagerConnection,
) field.ErrorList {
	path := field.NewPath("metadata").Child("name")
	dependents, errs := listConnectionDependents(ctx, reader, conn.Namespace, conn.Name)
	if len(errs) > 0 {
		return errs
	}
	if len(dependents) == 0 {
		return nil
	}
	return field.ErrorList{
		field.Invalid(path, conn.Name, fmt.Sprintf(
			"cannot delete QueueManagerConnection %q: %s; delete or re-point dependents first",
			conn.Name, formatDependents(dependents),
		)),
	}
}

type connectionDependent struct {
	kind string
	name string
}

func listConnectionDependents(
	ctx context.Context,
	reader client.Reader,
	namespace, connName string,
) ([]connectionDependent, field.ErrorList) {
	path := field.NewPath("metadata").Child("name")
	var dependents []connectionDependent

	var queues messagingv1alpha1.QueueList
	if err := reader.List(ctx, &queues, client.InNamespace(namespace)); err != nil {
		return nil, field.ErrorList{
			field.InternalError(path, fmt.Errorf("list Queues: %w", err)),
		}
	}
	for i := range queues.Items {
		if queues.Items[i].Spec.ConnectionRef.Name == connName {
			dependents = append(dependents, connectionDependent{kind: "Queue", name: queues.Items[i].Name})
		}
	}

	var topics messagingv1alpha1.TopicList
	if err := reader.List(ctx, &topics, client.InNamespace(namespace)); err != nil {
		return nil, field.ErrorList{
			field.InternalError(path, fmt.Errorf("list Topics: %w", err)),
		}
	}
	for i := range topics.Items {
		if topics.Items[i].Spec.ConnectionRef.Name == connName {
			dependents = append(dependents, connectionDependent{kind: "Topic", name: topics.Items[i].Name})
		}
	}

	var channels messagingv1alpha1.ChannelList
	if err := reader.List(ctx, &channels, client.InNamespace(namespace)); err != nil {
		return nil, field.ErrorList{
			field.InternalError(path, fmt.Errorf("list Channels: %w", err)),
		}
	}
	for i := range channels.Items {
		if channels.Items[i].Spec.ConnectionRef.Name == connName {
			dependents = append(dependents, connectionDependent{kind: "Channel", name: channels.Items[i].Name})
		}
	}

	var authRules messagingv1alpha1.ChannelAuthRuleList
	if err := reader.List(ctx, &authRules, client.InNamespace(namespace)); err != nil {
		return nil, field.ErrorList{
			field.InternalError(path, fmt.Errorf("list ChannelAuthRules: %w", err)),
		}
	}
	for i := range authRules.Items {
		if authRules.Items[i].Spec.ConnectionRef.Name == connName {
			dependents = append(dependents, connectionDependent{kind: "ChannelAuthRule", name: authRules.Items[i].Name})
		}
	}

	var authRecs messagingv1alpha1.AuthorityRecordList
	if err := reader.List(ctx, &authRecs, client.InNamespace(namespace)); err != nil {
		return nil, field.ErrorList{
			field.InternalError(path, fmt.Errorf("list AuthorityRecords: %w", err)),
		}
	}
	for i := range authRecs.Items {
		if authRecs.Items[i].Spec.ConnectionRef.Name == connName {
			dependents = append(dependents, connectionDependent{kind: "AuthorityRecord", name: authRecs.Items[i].Name})
		}
	}
	return dependents, nil
}

func formatDependents(dependents []connectionDependent) string {
	parts := make([]string, 0, len(dependents))
	for _, d := range dependents {
		parts = append(parts, fmt.Sprintf("%s %q", d.kind, d.name))
	}
	return strings.Join(parts, ", ")
}

func getSecretOrErrors(
	ctx context.Context,
	reader client.Reader,
	namespace, name string,
	path *field.Path,
) (field.ErrorList, *corev1.Secret) {
	secret := &corev1.Secret{}
	key := client.ObjectKey{Namespace: namespace, Name: name}
	if err := reader.Get(ctx, key, secret); err != nil {
		if apierrors.IsNotFound(err) {
			return field.ErrorList{
				field.NotFound(path, fmt.Sprintf("Secret %q not found in namespace %q", name, namespace)),
			}, nil
		}
		return field.ErrorList{field.InternalError(path, fmt.Errorf("get Secret %q: %w", name, err))}, nil
	}
	return nil, secret
}

func validateSecretExists(
	ctx context.Context,
	reader client.Reader,
	namespace, name string,
	path *field.Path,
) field.ErrorList {
	errs, _ := getSecretOrErrors(ctx, reader, namespace, name, path)
	return errs
}
