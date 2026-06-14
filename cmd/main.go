package main

import (
	"crypto/tls"
	"flag"
	"os"
	"strconv"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	messagingv1alpha1 "github.com/conduit-ops/mkurator/api/v1alpha1"
	"github.com/conduit-ops/mkurator/internal/adapter/mqrest"
	"github.com/conduit-ops/mkurator/internal/cacheconfig"
	"github.com/conduit-ops/mkurator/internal/controller"
	"github.com/conduit-ops/mkurator/internal/health"
	"github.com/conduit-ops/mkurator/internal/logging"
	webhookv1alpha1 "github.com/conduit-ops/mkurator/internal/webhook/v1alpha1"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(messagingv1alpha1.AddToScheme(scheme))

	// +kubebuilder:scaffold:scheme
}

// nolint:gocyclo
func main() {
	var metricsAddr string
	var metricsCertPath, metricsCertName, metricsCertKey string
	var webhookCertPath, webhookCertName, webhookCertKey string
	var enableLeaderElection bool
	var probeAddr string
	var secureMetrics bool
	var enableHTTP2 bool
	var logConfigPath string
	var logLevel string
	var logFormat string
	var maxConcurrentReconciles int
	var driftResyncLower time.Duration
	var driftResyncUpper time.Duration
	var connectionWaitInterval time.Duration
	var transientRequeueInterval time.Duration
	var mqRequestTimeout time.Duration
	var tlsOpts []func(*tls.Config)
	if v := os.Getenv("KURATOR_MAX_CONCURRENT_RECONCILES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxConcurrentReconciles = n
		}
	}
	flag.StringVar(&metricsAddr, "metrics-bind-address", "0", "The address the metrics endpoint binds to. "+
		"Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable the metrics service.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", true,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	flag.StringVar(&webhookCertPath, "webhook-cert-path", "", "The directory that contains the webhook certificate.")
	flag.StringVar(&webhookCertName, "webhook-cert-name", "tls.crt", "The name of the webhook certificate file.")
	flag.StringVar(&webhookCertKey, "webhook-cert-key", "tls.key", "The name of the webhook key file.")
	flag.StringVar(&metricsCertPath, "metrics-cert-path", "",
		"The directory that contains the metrics server certificate.")
	flag.StringVar(&metricsCertName, "metrics-cert-name", "tls.crt", "The name of the metrics server certificate file.")
	flag.StringVar(&metricsCertKey, "metrics-cert-key", "tls.key", "The name of the metrics server key file.")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	flag.StringVar(&logConfigPath, "log-config", "",
		"Path to a YAML logging config file (see docs/LOGGING.md).")
	flag.StringVar(&logLevel, "log-level", "",
		"Log level: debug, info, warn, or error (overrides env and file).")
	flag.StringVar(&logFormat, "log-format", "",
		"Log format: json or text (overrides env and file).")
	flag.IntVar(&maxConcurrentReconciles, "max-concurrent-reconciles", maxConcurrentReconciles,
		"Max concurrent reconcile workers per controller (also KURATOR_MAX_CONCURRENT_RECONCILES).")
	flag.DurationVar(&driftResyncLower, "drift-resync-min", 5*time.Minute,
		"Minimum jittered RequeueAfter for successfully synced workload CRs.")
	flag.DurationVar(&driftResyncUpper, "drift-resync-max", 10*time.Minute,
		"Maximum jittered RequeueAfter for successfully synced workload CRs.")
	flag.DurationVar(&connectionWaitInterval, "connection-wait-interval", 15*time.Second,
		"RequeueAfter while waiting for a QueueManagerConnection to become Ready.")
	flag.DurationVar(&transientRequeueInterval, "transient-requeue-interval", 30*time.Second,
		"RequeueAfter after transient MQ or connection errors.")
	flag.DurationVar(&mqRequestTimeout, "mq-request-timeout", 30*time.Second,
		"Per-request deadline for mqweb Admin calls from reconcilers.")
	flag.Parse()

	controller.SetMaxConcurrentReconciles(maxConcurrentReconciles)
	controller.SetDriftResyncInterval(driftResyncLower, driftResyncUpper)
	controller.SetConnectionWaitInterval(connectionWaitInterval)
	controller.SetTransientRequeueInterval(transientRequeueInterval)
	controller.SetMQRequestTimeout(mqRequestTimeout)

	logCfg, err := logging.Load(logging.Options{
		ConfigPath: logConfigPath,
		Level:      logLevel,
		Format:     logFormat,
	})
	if err != nil {
		setupLog.Error(err, "invalid logging configuration")
		os.Exit(1)
	}
	if err = logging.Setup(logCfg); err != nil {
		setupLog.Error(err, "configure logging")
		os.Exit(1)
	}
	setupLog = ctrl.Log.WithName("setup")
	setupLog.Info("logging configured", "level", logCfg.Level, "format", logCfg.Format)

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("Disabling HTTP/2")
		c.NextProtos = []string{"http/1.1"}
	}

	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	// Initial webhook TLS options
	webhookTLSOpts := tlsOpts
	webhookServerOptions := webhook.Options{
		TLSOpts: webhookTLSOpts,
	}

	if len(webhookCertPath) > 0 {
		setupLog.Info(
			"Initializing webhook certificate watcher using provided certificates",
			"webhook-cert-path",
			webhookCertPath,
			"webhook-cert-name",
			webhookCertName,
			"webhook-cert-key",
			webhookCertKey,
		)

		webhookServerOptions.CertDir = webhookCertPath
		webhookServerOptions.CertName = webhookCertName
		webhookServerOptions.KeyName = webhookCertKey
	}

	webhookServer := webhook.NewServer(webhookServerOptions)

	// Metrics endpoint is enabled in 'config/default/kustomization.yaml'. The Metrics options configure the server.
	// More info:
	// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.23.3/pkg/metrics/server
	// - https://book.kubebuilder.io/reference/metrics.html
	metricsServerOptions := metricsserver.Options{
		BindAddress:   metricsAddr,
		SecureServing: secureMetrics,
		TLSOpts:       tlsOpts,
	}

	if secureMetrics {
		// FilterProvider is used to protect the metrics endpoint with authn/authz.
		// These configurations ensure that only authorized users and service accounts
		// can access the metrics endpoint. The RBAC are configured in 'config/rbac/kustomization.yaml'. More info:
		// https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.23.3/pkg/metrics/filters#WithAuthenticationAndAuthorization
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	// If the certificate is not specified, controller-runtime will automatically
	// generate self-signed certificates for the metrics server. While convenient for development and testing,
	// this setup is not recommended for production.
	//
	// TODO(user): If you enable certManager, uncomment the following lines:
	// - [METRICS-WITH-CERTS] at config/default/kustomization.yaml to generate and use certificates
	// managed by cert-manager for the metrics server.
	// - [PROMETHEUS-WITH-CERTS] at config/prometheus/kustomization.yaml for TLS certification.
	if len(metricsCertPath) > 0 {
		setupLog.Info(
			"Initializing metrics certificate watcher using provided certificates",
			"metrics-cert-path",
			metricsCertPath,
			"metrics-cert-name",
			metricsCertName,
			"metrics-cert-key",
			metricsCertKey,
		)

		metricsServerOptions.CertDir = metricsCertPath
		metricsServerOptions.CertName = metricsCertName
		metricsServerOptions.KeyName = metricsCertKey
	}

	secretCacheOpts, secretClientOpts := cacheconfig.ManagerOptions()
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Cache:                  secretCacheOpts,
		Client:                 secretClientOpts,
		Metrics:                metricsServerOptions,
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "bdd44880.mkurator.dev",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "Failed to start manager")
		os.Exit(1)
	}

	// +kubebuilder:scaffold:builder

	mqFactory := mqrest.NewClientFactory(mgr.GetClient())
	eventRecorder := mgr.GetEventRecorder("mkurator-controller-manager")
	if err := (&controller.QueueManagerConnectionReconciler{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		MQFactory: mqFactory,
		Recorder:  eventRecorder,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "QueueManagerConnection")
		os.Exit(1)
	}
	if err := (&controller.QueueReconciler{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		MQFactory: mqFactory,
		Recorder:  eventRecorder,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Queue")
		os.Exit(1)
	}
	if err := (&controller.TopicReconciler{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		MQFactory: mqFactory,
		Recorder:  eventRecorder,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Topic")
		os.Exit(1)
	}
	if err := (&controller.ChannelReconciler{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		MQFactory: mqFactory,
		Recorder:  eventRecorder,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Channel")
		os.Exit(1)
	}
	if err := (&controller.ChannelAuthRuleReconciler{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		MQFactory: mqFactory,
		Recorder:  eventRecorder,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ChannelAuthRule")
		os.Exit(1)
	}
	if err := (&controller.AuthorityRecordReconciler{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		MQFactory: mqFactory,
		Recorder:  eventRecorder,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "AuthorityRecord")
		os.Exit(1)
	}

	if err := webhookv1alpha1.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to setup webhooks")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "Failed to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", health.NewMQConnectivityChecker(mgr.GetClient())); err != nil {
		setupLog.Error(err, "Failed to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("Starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "Failed to run manager")
		os.Exit(1)
	}
}
