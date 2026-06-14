package webhookv1alpha1

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	messagingv1alpha1 "github.com/conduit-ops/mkurator/api/v1alpha1"
)

var (
	webhookTestEnv   *envtest.Environment
	webhookCfg       *rest.Config
	webhookK8sClient client.Client
	webhookCancel    context.CancelFunc
)

func TestWebhookAdmission(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Webhook Admission Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping webhook test environment")
	webhookTestEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "..", "config", "crd", "bases"),
		},
		WebhookInstallOptions: envtest.WebhookInstallOptions{
			Paths: []string{
				filepath.Join("..", "..", "..", "config", "webhook", "manifests.yaml"),
			},
		},
	}

	var err error
	webhookCfg, err = webhookTestEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(messagingv1alpha1.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(corev1.AddToScheme(scheme.Scheme)).To(Succeed())

	webhookK8sClient, err = client.New(webhookCfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())

	mgr, err := ctrl.NewManager(webhookCfg, ctrl.Options{
		Scheme:  scheme.Scheme,
		Metrics: metricsserver.Options{BindAddress: "0"},
		WebhookServer: webhook.NewServer(webhook.Options{
			Host:    webhookTestEnv.WebhookInstallOptions.LocalServingHost,
			Port:    webhookTestEnv.WebhookInstallOptions.LocalServingPort,
			CertDir: webhookTestEnv.WebhookInstallOptions.LocalServingCertDir,
		}),
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(SetupWithManager(mgr)).To(Succeed())

	ctx, cancel := context.WithCancel(context.Background())
	webhookCancel = cancel
	go func() {
		defer GinkgoRecover()
		Expect(mgr.Start(ctx)).To(Succeed())
	}()
	time.Sleep(2 * time.Second)
})

var _ = AfterSuite(func() {
	if webhookCancel != nil {
		webhookCancel()
	}
	Expect(webhookTestEnv.Stop()).To(Succeed())
})

var _ = Describe("Validating admission webhooks", func() {
	const ns = "webhook-admission"

	BeforeEach(func() {
		ctx := context.Background()
		nsObj := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}
		_ = webhookK8sClient.Create(ctx, nsObj)
		cleanupWebhookNamespace(ctx, ns)
	})

	AfterEach(func() {
		cleanupWebhookNamespace(context.Background(), ns)
	})

	It("denies Queue when connectionRef target is missing", func() {
		ctx := context.Background()
		q := &messagingv1alpha1.Queue{
			ObjectMeta: metav1.ObjectMeta{Name: "bad-queue", Namespace: ns},
			Spec: messagingv1alpha1.QueueSpec{
				ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "missing-qmc"},
				QueueName:     "APP.ORDERS",
			},
		}
		err := webhookK8sClient.Create(ctx, q)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())
	})

	It("denies alias Queue without targq", func() {
		ctx := context.Background()
		Expect(webhookK8sClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "creds", Namespace: ns},
		})).To(Succeed())
		conn := sampleWebhookConnection(ns, "qm1")
		Expect(webhookK8sClient.Create(ctx, conn)).To(Succeed())

		q := &messagingv1alpha1.Queue{
			ObjectMeta: metav1.ObjectMeta{Name: "alias-queue", Namespace: ns},
			Spec: messagingv1alpha1.QueueSpec{
				ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
				QueueName:     "ALIAS.Q",
				Type:          messagingv1alpha1.QueueTypeAlias,
			},
		}
		err := webhookK8sClient.Create(ctx, q)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())
	})

	It("allows valid Queue when connection and spec are valid", func() {
		ctx := context.Background()
		Expect(webhookK8sClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "creds", Namespace: ns},
		})).To(Succeed())
		conn := sampleWebhookConnection(ns, "qm1")
		Expect(webhookK8sClient.Create(ctx, conn)).To(Succeed())

		q := &messagingv1alpha1.Queue{
			ObjectMeta: metav1.ObjectMeta{Name: "good-queue", Namespace: ns},
			Spec: messagingv1alpha1.QueueSpec{
				ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
				QueueName:     "APP.ORDERS",
				Attributes:    map[string]string{"maxdepth": "1000", "descr": "Orders intake queue"},
			},
		}
		Expect(webhookK8sClient.Create(ctx, q)).To(Succeed())
	})

	It("denies QueueManagerConnection with insecure TLS without opt-in annotation", func() {
		ctx := context.Background()
		Expect(webhookK8sClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "creds", Namespace: ns},
		})).To(Succeed())

		conn := sampleWebhookConnection(ns, "insecure-qmc")
		conn.Spec.TLS = &messagingv1alpha1.TLSConfig{InsecureSkipVerify: true}
		err := webhookK8sClient.Create(ctx, conn)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())
		Expect(err.Error()).To(ContainSubstring(messagingv1alpha1.AllowInsecureTLSAnnotation))
	})

	It("allows QueueManagerConnection with insecure TLS when opt-in annotation is set", func() {
		ctx := context.Background()
		Expect(webhookK8sClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "creds", Namespace: ns},
		})).To(Succeed())

		conn := sampleWebhookConnection(ns, "insecure-qmc-ok")
		conn.Annotations = map[string]string{
			messagingv1alpha1.AllowInsecureTLSAnnotation: "true",
		}
		conn.Spec.TLS = &messagingv1alpha1.TLSConfig{InsecureSkipVerify: true}
		Expect(webhookK8sClient.Create(ctx, conn)).To(Succeed())
	})

	It("denies QueueManagerConnection delete when dependents exist", func() {
		ctx := context.Background()
		Expect(webhookK8sClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "creds", Namespace: ns},
		})).To(Succeed())
		conn := sampleWebhookConnection(ns, "qm1")
		Expect(webhookK8sClient.Create(ctx, conn)).To(Succeed())

		q := &messagingv1alpha1.Queue{
			ObjectMeta: metav1.ObjectMeta{Name: "dep-queue", Namespace: ns},
			Spec: messagingv1alpha1.QueueSpec{
				ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
				QueueName:     "APP.ORDERS",
			},
		}
		Expect(webhookK8sClient.Create(ctx, q)).To(Succeed())

		err := webhookK8sClient.Delete(ctx, conn)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())
		Expect(err.Error()).To(ContainSubstring("dep-queue"))
	})

	It("denies ChannelAuthRule when managed Channel is missing", func() {
		ctx := context.Background()
		Expect(webhookK8sClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "creds", Namespace: ns},
		})).To(Succeed())
		conn := sampleWebhookConnection(ns, "qm1")
		Expect(webhookK8sClient.Create(ctx, conn)).To(Succeed())

		rule := &messagingv1alpha1.ChannelAuthRule{
			ObjectMeta: metav1.ObjectMeta{Name: "bad-car", Namespace: ns},
			Spec: messagingv1alpha1.ChannelAuthRuleSpec{
				ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
				ChannelName:   "ORDERS.APP",
				RuleType:      messagingv1alpha1.ChannelAuthRuleTypeAddressMap,
				Address:       "*",
			},
		}
		err := webhookK8sClient.Create(ctx, rule)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())
		Expect(err.Error()).To(ContainSubstring("ORDERS.APP"))
	})

	It("denies ChannelAuthRule BLOCKADDR without address", func() {
		ctx := context.Background()
		Expect(webhookK8sClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "creds", Namespace: ns},
		})).To(Succeed())
		conn := sampleWebhookConnection(ns, "qm1")
		Expect(webhookK8sClient.Create(ctx, conn)).To(Succeed())

		ch := &messagingv1alpha1.Channel{
			ObjectMeta: metav1.ObjectMeta{Name: "orders-app", Namespace: ns},
			Spec: messagingv1alpha1.ChannelSpec{
				ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
				ChannelName:   "ORDERS.APP",
			},
		}
		Expect(webhookK8sClient.Create(ctx, ch)).To(Succeed())

		rule := &messagingv1alpha1.ChannelAuthRule{
			ObjectMeta: metav1.ObjectMeta{Name: "bad-blockaddr", Namespace: ns},
			Spec: messagingv1alpha1.ChannelAuthRuleSpec{
				ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
				ChannelName:   "ORDERS.APP",
				RuleType:      messagingv1alpha1.ChannelAuthRuleTypeBlockAddr,
			},
		}
		err := webhookK8sClient.Create(ctx, rule)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())
		Expect(err.Error()).To(ContainSubstring("address"))
	})

	It("allows ChannelAuthRule USERMAP when Channel exists (deferred MQSC fields)", func() {
		ctx := context.Background()
		Expect(webhookK8sClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "creds", Namespace: ns},
		})).To(Succeed())
		conn := sampleWebhookConnection(ns, "qm1")
		Expect(webhookK8sClient.Create(ctx, conn)).To(Succeed())

		ch := &messagingv1alpha1.Channel{
			ObjectMeta: metav1.ObjectMeta{Name: "orders-app", Namespace: ns},
			Spec: messagingv1alpha1.ChannelSpec{
				ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
				ChannelName:   "ORDERS.APP",
			},
		}
		Expect(webhookK8sClient.Create(ctx, ch)).To(Succeed())

		rule := &messagingv1alpha1.ChannelAuthRule{
			ObjectMeta: metav1.ObjectMeta{Name: "car-usermap", Namespace: ns},
			Spec: messagingv1alpha1.ChannelAuthRuleSpec{
				ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
				ChannelName:   "ORDERS.APP",
				RuleType:      messagingv1alpha1.ChannelAuthRuleTypeUserMap,
			},
		}
		Expect(webhookK8sClient.Create(ctx, rule)).To(Succeed())
	})

	It("allows ChannelAuthRule SSLPEERMAP when Channel exists (deferred MQSC fields)", func() {
		ctx := context.Background()
		Expect(webhookK8sClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "creds", Namespace: ns},
		})).To(Succeed())
		conn := sampleWebhookConnection(ns, "qm1")
		Expect(webhookK8sClient.Create(ctx, conn)).To(Succeed())

		ch := &messagingv1alpha1.Channel{
			ObjectMeta: metav1.ObjectMeta{Name: "orders-app", Namespace: ns},
			Spec: messagingv1alpha1.ChannelSpec{
				ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
				ChannelName:   "ORDERS.APP",
			},
		}
		Expect(webhookK8sClient.Create(ctx, ch)).To(Succeed())

		rule := &messagingv1alpha1.ChannelAuthRule{
			ObjectMeta: metav1.ObjectMeta{Name: "car-sslpeermap", Namespace: ns},
			Spec: messagingv1alpha1.ChannelAuthRuleSpec{
				ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
				ChannelName:   "ORDERS.APP",
				RuleType:      messagingv1alpha1.ChannelAuthRuleTypeSSLPeerMap,
			},
		}
		Expect(webhookK8sClient.Create(ctx, rule)).To(Succeed())
	})

	It("allows ChannelAuthRule when Channel and connection exist", func() {
		ctx := context.Background()
		Expect(webhookK8sClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "creds", Namespace: ns},
		})).To(Succeed())
		conn := sampleWebhookConnection(ns, "qm1")
		Expect(webhookK8sClient.Create(ctx, conn)).To(Succeed())

		ch := &messagingv1alpha1.Channel{
			ObjectMeta: metav1.ObjectMeta{Name: "orders-app", Namespace: ns},
			Spec: messagingv1alpha1.ChannelSpec{
				ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
				ChannelName:   "ORDERS.APP",
			},
		}
		Expect(webhookK8sClient.Create(ctx, ch)).To(Succeed())

		rule := &messagingv1alpha1.ChannelAuthRule{
			ObjectMeta: metav1.ObjectMeta{Name: "good-car", Namespace: ns},
			Spec: messagingv1alpha1.ChannelAuthRuleSpec{
				ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
				ChannelName:   "ORDERS.APP",
				RuleType:      messagingv1alpha1.ChannelAuthRuleTypeAddressMap,
				Address:       "*",
			},
		}
		Expect(webhookK8sClient.Create(ctx, rule)).To(Succeed())
	})

	// T6 (EC-P1-04): MQSC injection hardening — admission must reject values that would
	// break out of USERSRC/CHCKCLNT/AUTHADD MQSC tokens.
	Describe("MQSC injection hardening (T6)", func() {
		BeforeEach(func() {
			ctx := context.Background()
			Expect(webhookK8sClient.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "creds", Namespace: ns},
			})).To(Succeed())
			Expect(webhookK8sClient.Create(ctx, sampleWebhookConnection(ns, "qm1"))).To(Succeed())
			ch := &messagingv1alpha1.Channel{
				ObjectMeta: metav1.ObjectMeta{Name: "orders-app", Namespace: ns},
				Spec: messagingv1alpha1.ChannelSpec{
					ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
					ChannelName:   "ORDERS.APP",
				},
			}
			Expect(webhookK8sClient.Create(ctx, ch)).To(Succeed())
		})

		It("denies ChannelAuthRule with MQSC injection in userSource", func() {
			ctx := context.Background()
			rule := &messagingv1alpha1.ChannelAuthRule{
				ObjectMeta: metav1.ObjectMeta{Name: "inject-usersrc", Namespace: ns},
				Spec: messagingv1alpha1.ChannelAuthRuleSpec{
					ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
					ChannelName:   "ORDERS.APP",
					RuleType:      messagingv1alpha1.ChannelAuthRuleTypeAddressMap,
					Address:       "*",
					UserSource:    messagingv1alpha1.ChannelAuthUserSource(`MAP) MCAUSER('mqm'`),
				},
			}
			err := webhookK8sClient.Create(ctx, rule)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsInvalid(err)).To(BeTrue())
			Expect(err.Error()).To(ContainSubstring("spec.userSource"))
		})

		It("denies ChannelAuthRule with MQSC injection in checkClient", func() {
			ctx := context.Background()
			rule := &messagingv1alpha1.ChannelAuthRule{
				ObjectMeta: metav1.ObjectMeta{Name: "inject-chckclnt", Namespace: ns},
				Spec: messagingv1alpha1.ChannelAuthRuleSpec{
					ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
					ChannelName:   "ORDERS.APP",
					RuleType:      messagingv1alpha1.ChannelAuthRuleTypeAddressMap,
					Address:       "*",
					CheckClient:   messagingv1alpha1.ChannelAuthCheckClient("REQUIRED) ACTION(REPLACE"),
				},
			}
			err := webhookK8sClient.Create(ctx, rule)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsInvalid(err)).To(BeTrue())
			Expect(err.Error()).To(ContainSubstring("spec.checkClient"))
		})

		It("denies AuthorityRecord with MQSC injection in authorities", func() {
			ctx := context.Background()
			auth := &messagingv1alpha1.AuthorityRecord{
				ObjectMeta: metav1.ObjectMeta{Name: "inject-auth", Namespace: ns},
				Spec: messagingv1alpha1.AuthorityRecordSpec{
					ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
					Profile:       "APP.ORDERS",
					ObjectType:    messagingv1alpha1.AuthorityObjectTypeQueue,
					Principal:     "app",
					Authorities:   []string{`GET) AUTHADD(ALL`},
				},
			}
			err := webhookK8sClient.Create(ctx, auth)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsInvalid(err)).To(BeTrue())
			Expect(err.Error()).To(ContainSubstring("spec.authorities"))
		})
	})

	It("denies QueueManagerConnection delete when ChannelAuthRule dependent exists", func() {
		ctx := context.Background()
		Expect(webhookK8sClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "creds", Namespace: ns},
		})).To(Succeed())
		conn := sampleWebhookConnection(ns, "qm1")
		Expect(webhookK8sClient.Create(ctx, conn)).To(Succeed())

		ch := &messagingv1alpha1.Channel{
			ObjectMeta: metav1.ObjectMeta{Name: "orders-app", Namespace: ns},
			Spec: messagingv1alpha1.ChannelSpec{
				ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
				ChannelName:   "ORDERS.APP",
			},
		}
		Expect(webhookK8sClient.Create(ctx, ch)).To(Succeed())

		rule := &messagingv1alpha1.ChannelAuthRule{
			ObjectMeta: metav1.ObjectMeta{Name: "dep-car", Namespace: ns},
			Spec: messagingv1alpha1.ChannelAuthRuleSpec{
				ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
				ChannelName:   "ORDERS.APP",
				RuleType:      messagingv1alpha1.ChannelAuthRuleTypeAddressMap,
				Address:       "*",
			},
		}
		Expect(webhookK8sClient.Create(ctx, rule)).To(Succeed())

		err := webhookK8sClient.Delete(ctx, conn)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())
		Expect(err.Error()).To(ContainSubstring("dep-car"))
	})

	It("allows QueueManagerConnection delete after dependents removed", func() {
		ctx := context.Background()
		Expect(webhookK8sClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "creds", Namespace: ns},
		})).To(Succeed())
		conn := sampleWebhookConnection(ns, "qm1")
		Expect(webhookK8sClient.Create(ctx, conn)).To(Succeed())

		q := &messagingv1alpha1.Queue{
			ObjectMeta: metav1.ObjectMeta{Name: "dep-queue", Namespace: ns},
			Spec: messagingv1alpha1.QueueSpec{
				ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
				QueueName:     "APP.ORDERS",
			},
		}
		Expect(webhookK8sClient.Create(ctx, q)).To(Succeed())
		Expect(webhookK8sClient.Delete(ctx, q)).To(Succeed())
		Expect(webhookK8sClient.Delete(ctx, conn)).To(Succeed())
	})

	// Requires envtest K8s ≥ 1.27 for admission warning propagation to the client.
	It("allows QueueManagerConnection create and warns when credentials Secret omits username", func() {
		ctx := context.Background()
		Expect(
			webhookK8sClient.Create(
				ctx,
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "creds-no-user", Namespace: ns},
					Data:       map[string][]byte{"password": []byte("x")},
				},
			),
		).To(Succeed())
		warningClient, capture := newWarningCapturingClient()
		conn := sampleWebhookConnection(ns, "qm-no-user")
		conn.Spec.CredentialsSecretRef.Name = "creds-no-user"
		Expect(warningClient.Create(ctx, conn)).To(Succeed())
		capture.mu.Lock()
		defer capture.mu.Unlock()
		combined := strings.Join(*capture.store, " ")
		Expect(*capture.store).NotTo(BeEmpty(), "expected admission warnings, got none")
		Expect(combined).To(ContainSubstring(`default to "admin"`))
	})

	Describe("unknown spec.attributes admission warnings", func() {
		BeforeEach(func() {
			ctx := context.Background()
			Expect(webhookK8sClient.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "creds", Namespace: ns},
			})).To(Succeed())
			Expect(webhookK8sClient.Create(ctx, sampleWebhookConnection(ns, "qm1"))).To(Succeed())
		})

		It("allows Queue create and warns on unknown attribute keys", func() {
			ctx := context.Background()
			warningClient, capture := newWarningCapturingClient()
			q := &messagingv1alpha1.Queue{
				ObjectMeta: metav1.ObjectMeta{Name: "warn-queue", Namespace: ns},
				Spec: messagingv1alpha1.QueueSpec{
					ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
					QueueName:     "APP.WARN",
					Attributes:    map[string]string{"notreal": "x"},
				},
			}
			Expect(warningClient.Create(ctx, q)).To(Succeed())
			expectUnknownAttributeWarning(capture, "notreal")
		})

		It("allows Topic create and warns on unknown attribute keys", func() {
			ctx := context.Background()
			warningClient, capture := newWarningCapturingClient()
			topic := &messagingv1alpha1.Topic{
				ObjectMeta: metav1.ObjectMeta{Name: "warn-topic", Namespace: ns},
				Spec: messagingv1alpha1.TopicSpec{
					ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
					TopicName:     "RETAIL/ORDERS",
					Attributes:    map[string]string{"boguskey": "y"},
				},
			}
			Expect(warningClient.Create(ctx, topic)).To(Succeed())
			expectUnknownAttributeWarning(capture, "boguskey")
		})

		It("allows Channel create and warns on unknown attribute keys", func() {
			ctx := context.Background()
			warningClient, capture := newWarningCapturingClient()
			ch := &messagingv1alpha1.Channel{
				ObjectMeta: metav1.ObjectMeta{Name: "warn-channel", Namespace: ns},
				Spec: messagingv1alpha1.ChannelSpec{
					ConnectionRef: messagingv1alpha1.LocalObjectReference{Name: "qm1"},
					ChannelName:   "ORDERS.WARN",
					Attributes:    map[string]string{"unknownattr": "z"},
				},
			}
			Expect(warningClient.Create(ctx, ch)).To(Succeed())
			expectUnknownAttributeWarning(capture, "unknownattr")
		})
	})
})

func sampleWebhookConnection(ns, name string) *messagingv1alpha1.QueueManagerConnection {
	return &messagingv1alpha1.QueueManagerConnection{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: messagingv1alpha1.QueueManagerConnectionSpec{
			QueueManager: "QM1",
			Endpoint:     "https://mq.example:9443",
			CredentialsSecretRef: messagingv1alpha1.SecretReference{
				Name: "creds",
			},
		},
	}
}

func cleanupWebhookNamespace(ctx context.Context, ns string) {
	_ = webhookK8sClient.DeleteAllOf(ctx, &messagingv1alpha1.Queue{}, client.InNamespace(ns))
	_ = webhookK8sClient.DeleteAllOf(ctx, &messagingv1alpha1.Topic{}, client.InNamespace(ns))
	_ = webhookK8sClient.DeleteAllOf(ctx, &messagingv1alpha1.ChannelAuthRule{}, client.InNamespace(ns))
	_ = webhookK8sClient.DeleteAllOf(ctx, &messagingv1alpha1.AuthorityRecord{}, client.InNamespace(ns))
	_ = webhookK8sClient.DeleteAllOf(ctx, &messagingv1alpha1.Channel{}, client.InNamespace(ns))
	_ = webhookK8sClient.DeleteAllOf(ctx, &messagingv1alpha1.QueueManagerConnection{}, client.InNamespace(ns))
	_ = webhookK8sClient.DeleteAllOf(ctx, &corev1.Secret{}, client.InNamespace(ns))
}

type warningCapture struct {
	store *[]string
	mu    *sync.Mutex
}

func (w warningCapture) HandleWarningHeader(_ int, _ string, text string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	*w.store = append(*w.store, text)
}

func newWarningCapturingClient() (client.Client, *warningCapture) {
	var (
		mu       sync.Mutex
		warnings []string
	)
	capture := &warningCapture{store: &warnings, mu: &mu}
	warningCfg := rest.CopyConfig(webhookCfg)
	warningCfg.WarningHandler = capture
	warningClient, err := client.New(warningCfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	return warningClient, capture
}

func expectUnknownAttributeWarning(capture *warningCapture, attrKey string) {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	combined := strings.Join(*capture.store, " ")
	Expect(*capture.store).NotTo(BeEmpty(), "expected admission warnings, got none")
	Expect(combined).To(ContainSubstring(attrKey))
	Expect(combined).To(ContainSubstring("drift-check allow-list"))
}
