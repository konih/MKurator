//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/conduit-ops/mkurator/internal/adapter/mqrest"
	"github.com/conduit-ops/mkurator/internal/mqadmin"
)

const e2eChannelName = "DEV.APP.SVRCONN.0TLS"

// mqE2EEnabled reports whether IBM MQ integration tests should run.
func mqE2EEnabled() bool {
	return os.Getenv("KURATOR_E2E_MQ") == "1"
}

func mqE2EConfig() (mqrest.Config, error) {
	endpoint := envOr("KURATOR_E2E_MQ_ENDPOINT", "https://127.0.0.1:30443")
	u, err := url.Parse(endpoint)
	if err != nil {
		return mqrest.Config{}, fmt.Errorf("parse KURATOR_E2E_MQ_ENDPOINT: %w", err)
	}

	tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if envOr("KURATOR_E2E_MQ_INSECURE_TLS", "true") == "true" {
		tlsCfg.InsecureSkipVerify = true //nolint:gosec // local kind / mkcert only
	}

	host := envOr("KURATOR_E2E_MQ_HOST", "mq.localhost")
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsCfg

	return mqrest.Config{
		Endpoint:     u,
		QueueManager: envOr("KURATOR_E2E_MQ_QMGR", "QM1"),
		Username:     envOr("KURATOR_E2E_MQ_USER", "admin"),
		Password:     envOr("KURATOR_E2E_MQ_PASSWORD", "passw0rd"),
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
			Transport: &hostHeaderRoundTripper{
				host:      host,
				transport: transport,
			},
		},
	}, nil
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

func newMQClient() (*mqrest.Client, error) {
	cfg, err := mqE2EConfig()
	if err != nil {
		return nil, err
	}
	return mqrest.NewClient(cfg)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func fixturePath(name string) (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "fixtures", name), nil
}

// applyMQSCFixture reads an MQSC file (comments and blank lines skipped) and runs each command.
func applyMQSCFixture(ctx context.Context, client *mqrest.Client, filename string) error {
	path, err := fixturePath(filename)
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read fixture %s: %w", filename, err)
	}
	for line := range strings.Lines(string(raw)) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "*") {
			continue
		}
		if err := client.RunMQSC(ctx, line); err != nil {
			return fmt.Errorf("mqsc %q: %w", line, err)
		}
	}
	return nil
}

func topicExists(ctx context.Context, client *mqrest.Client, name string) (bool, error) {
	_, err := client.GetTopic(ctx, name)
	if err == nil {
		return true, nil
	}
	if strings.Contains(strings.ToUpper(err.Error()), "AMQ8147") ||
		strings.Contains(strings.ToLower(err.Error()), "not found") {
		return false, nil
	}
	return false, err
}

func svrconnChannelExists(ctx context.Context, client *mqrest.Client, name string) (bool, error) {
	_, err := client.GetChannel(ctx, mqadmin.ChannelSpec{Name: name, Type: mqadmin.ChannelTypeSvrconn})
	if err == nil {
		return true, nil
	}
	if strings.Contains(strings.ToUpper(err.Error()), "AMQ8147") ||
		strings.Contains(strings.ToLower(err.Error()), "not found") {
		return false, nil
	}
	return false, err
}

// channelExists returns true when DISPLAY CHANNEL succeeds for the named SVRCONN channel.
func channelExists(ctx context.Context, client *mqrest.Client, name string) (bool, error) {
	cmd := fmt.Sprintf("DISPLAY CHANNEL('%s') CHLTYPE(SVRCONN)", strings.ReplaceAll(name, "'", "''"))
	err := client.RunMQSC(ctx, cmd)
	if err == nil {
		return true, nil
	}
	if strings.Contains(strings.ToUpper(err.Error()), "AMQ8147") ||
		strings.Contains(strings.ToLower(err.Error()), "not found") {
		return false, nil
	}
	return false, err
}

// channelAuthExists reports whether a CHLAUTH rule is present on MQ (adapter GET path).
func channelAuthExists(ctx context.Context, client *mqrest.Client, spec mqadmin.ChannelAuthSpec) (bool, error) {
	_, err := client.GetChannelAuth(ctx, spec)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, mqadmin.ErrNotFound) {
		return false, nil
	}
	return false, err
}

// channelAuthMatches reports whether observed CHLAUTH attributes match the desired spec.
func channelAuthMatches(ctx context.Context, client *mqrest.Client, spec mqadmin.ChannelAuthSpec) (bool, error) {
	state, err := client.GetChannelAuth(ctx, spec)
	if err != nil {
		if errors.Is(err, mqadmin.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return !mqadmin.ChannelAuthNeedsUpdate(spec, state), nil
}

// authorityExists reports whether an AUTHREC is present on MQ (adapter GET path).
func authorityExists(ctx context.Context, client *mqrest.Client, spec mqadmin.AuthoritySpec) (bool, error) {
	_, err := client.GetAuthority(ctx, spec)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, mqadmin.ErrNotFound) {
		return false, nil
	}
	return false, err
}

// authorityMatches reports whether observed OAM authorities match the desired spec.
func authorityMatches(ctx context.Context, client *mqrest.Client, spec mqadmin.AuthoritySpec) (bool, error) {
	state, err := client.GetAuthority(ctx, spec)
	if err != nil {
		if errors.Is(err, mqadmin.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return !mqadmin.AuthorityNeedsUpdate(spec, state), nil
}
