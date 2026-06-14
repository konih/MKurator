package samples_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	messagingv1alpha1 "github.com/conduit-ops/mkurator/api/v1alpha1"
	"github.com/conduit-ops/mkurator/internal/validation"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func sampleFiles(t *testing.T) []string {
	t.Helper()
	root := filepath.Join(repoRoot(t), "config", "samples")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read samples dir: %v", err)
	}
	var files []string
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		name := ent.Name()
		if !strings.HasPrefix(name, "messaging_v1alpha1_") || !strings.HasSuffix(name, ".yaml") {
			continue
		}
		files = append(files, filepath.Join(root, name))
	}
	return files
}

func loadScheme(t *testing.T) *k8sruntime.Scheme {
	t.Helper()
	s := k8sruntime.NewScheme()
	if err := messagingv1alpha1.AddToScheme(s); err != nil {
		t.Fatalf("add scheme: %v", err)
	}
	if err := corev1.AddToScheme(s); err != nil {
		t.Fatalf("add corev1: %v", err)
	}
	return s
}

func decodeObject(t *testing.T, path string) client.Object {
	t.Helper()
	//nolint:gosec // G304: path is under config/samples only
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	dec := yaml.NewYAMLOrJSONDecoder(strings.NewReader(string(data)), 4096)
	var meta struct {
		Kind string `json:"kind"`
	}
	if err := dec.Decode(&meta); err != nil {
		t.Fatalf("decode meta %s: %v", path, err)
	}

	dec = yaml.NewYAMLOrJSONDecoder(strings.NewReader(string(data)), 4096)

	switch meta.Kind {
	case "QueueManagerConnection":
		var obj messagingv1alpha1.QueueManagerConnection
		if err := dec.Decode(&obj); err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}
		return &obj
	case "Queue":
		var obj messagingv1alpha1.Queue
		if err := dec.Decode(&obj); err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}
		return &obj
	case "Topic":
		var obj messagingv1alpha1.Topic
		if err := dec.Decode(&obj); err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}
		return &obj
	case "Channel":
		var obj messagingv1alpha1.Channel
		if err := dec.Decode(&obj); err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}
		return &obj
	case "ChannelAuthRule":
		var obj messagingv1alpha1.ChannelAuthRule
		if err := dec.Decode(&obj); err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}
		return &obj
	case "AuthorityRecord":
		var obj messagingv1alpha1.AuthorityRecord
		if err := dec.Decode(&obj); err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}
		return &obj
	default:
		t.Fatalf("%s: unsupported kind %q", path, meta.Kind)
	}
	return nil
}

func TestConfigSamplesDecode(t *testing.T) {
	t.Parallel()
	for _, path := range sampleFiles(t) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			obj := decodeObject(t, path)
			if obj.GetName() == "" {
				t.Fatal("missing metadata.name")
			}
		})
	}
}

func TestConfigSamplesAdmissionValidation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := loadScheme(t)
	paths := sampleFiles(t)

	objects := make([]client.Object, 0, len(paths)+1)
	for _, path := range paths {
		objects = append(objects, decodeObject(t, path))
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "mq-credentials", Namespace: "mkurator-system"},
	}
	objects = append(objects, secret)

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()

	for _, obj := range objects {
		switch o := obj.(type) {
		case *messagingv1alpha1.QueueManagerConnection:
			_, errs := validation.ValidateQueueManagerConnectionSpec(
				ctx, cl, o.Namespace, o.Annotations, &o.Spec,
			)
			if len(errs) > 0 {
				t.Fatalf("QueueManagerConnection/%s: %v", o.Name, errs)
			}
		case *messagingv1alpha1.Queue:
			if _, errs := validation.ValidateQueueSpec(ctx, cl, o.Namespace, o.Name, &o.Spec); len(errs) > 0 {
				t.Fatalf("Queue/%s: %v", o.Name, errs)
			}
		case *messagingv1alpha1.Topic:
			if _, errs := validation.ValidateTopicSpec(ctx, cl, o.Namespace, o.Name, &o.Spec); len(errs) > 0 {
				t.Fatalf("Topic/%s: %v", o.Name, errs)
			}
		case *messagingv1alpha1.Channel:
			if _, errs := validation.ValidateChannelSpec(ctx, cl, o.Namespace, o.Name, &o.Spec); len(errs) > 0 {
				t.Fatalf("Channel/%s: %v", o.Name, errs)
			}
		case *messagingv1alpha1.ChannelAuthRule, *messagingv1alpha1.AuthorityRecord:
			// Auth samples illustrate MQ rule shapes; cross-CR channel profile checks are covered in envtest.
		case *corev1.Secret:
		default:
			t.Fatalf("unexpected type %T", obj)
		}
	}
}
