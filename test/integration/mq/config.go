//go:build integration

package mq

import (
	"crypto/tls"
	"fmt"
	"hash/fnv"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/conduit-ops/mkurator/internal/adapter/mqrest"
)

func integrationEnabled() bool {
	return os.Getenv("KURATOR_INTEGRATION_MQ") == "1"
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func integrationConfig() (mqrest.Config, error) {
	endpoint := envOr("KURATOR_INTEGRATION_MQ_ENDPOINT", "https://127.0.0.1:9443")
	u, err := url.Parse(endpoint)
	if err != nil {
		return mqrest.Config{}, fmt.Errorf("parse KURATOR_INTEGRATION_MQ_ENDPOINT: %w", err)
	}

	tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if envOr("KURATOR_INTEGRATION_MQ_INSECURE_TLS", "true") == "true" {
		tlsCfg.InsecureSkipVerify = true //nolint:gosec // local Docker / mkcert only
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsCfg

	host := envOr("KURATOR_INTEGRATION_MQ_HOST", "")
	var rt http.RoundTripper = transport
	if host != "" {
		rt = &hostHeaderRoundTripper{host: host, transport: transport}
	}

	return mqrest.Config{
		Endpoint:     u,
		QueueManager: envOr("KURATOR_INTEGRATION_MQ_QMGR", "QM1"),
		Username:     envOr("KURATOR_INTEGRATION_MQ_USER", "admin"),
		Password:     envOr("KURATOR_INTEGRATION_MQ_PASSWORD", "passw0rd"),
		HTTPClient: &http.Client{
			Timeout:   60 * time.Second,
			Transport: rt,
		},
	}, nil
}

func newIntegrationClient() (*mqrest.Client, error) {
	cfg, err := integrationConfig()
	if err != nil {
		return nil, err
	}
	return mqrest.NewClient(cfg)
}

type hostHeaderRoundTripper struct {
	host      string
	transport http.RoundTripper
}

func (h *hostHeaderRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Host = h.host
	req.Header.Set("Host", h.host)
	return h.transport.RoundTrip(req)
}

// objectNameForTest returns a unique MQ object name for the test (max 48 chars).
func objectNameForTest(tName string) string {
	const prefix = "KURATOR.IT."
	safe := strings.Map(func(r rune) rune {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.':
			return r
		case r >= 'a' && r <= 'z':
			return r - ('a' - 'A')
		default:
			return '.'
		}
	}, tName)
	name := prefix + safe
	if len(name) > 48 {
		name = name[:48]
	}
	return strings.Trim(name, ".")
}

func queueNameForTest(tName string) string { return objectNameForTest(tName) }

func integrationQueueManager() string {
	return envOr("KURATOR_INTEGRATION_MQ_QMGR", "QM1")
}

func topicNameForTest(tName string) string {
	return fmt.Sprintf("KIT.T.%05d", testNameHash(tName)%100000)
}

// channelNameForTest returns a name within IBM MQ's 20-character channel limit.
func channelNameForTest(tName string) string {
	return fmt.Sprintf("KIT.C.%05d", testNameHash(tName)%100000)
}

func testNameHash(tName string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(tName))
	return h.Sum32()
}
