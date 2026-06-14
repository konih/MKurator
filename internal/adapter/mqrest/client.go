package mqrest

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/conduit-ops/mkurator/internal/metrics"
	"github.com/conduit-ops/mkurator/internal/mqadmin"
)

const (
	// DefaultRESTPrefix is the mqweb REST API path for IBM MQ 9.3+.
	DefaultRESTPrefix  = "/ibmmq/rest/v3"
	csrfHeader         = "ibm-mq-rest-csrf-token"
	mqscType           = "runCommandJSON"
	mqscCommandDisplay = "display"
	mqscCommandDefine  = "define"
	mqscCommandDelete  = "delete"
	mqscReplaceYes     = "yes"
	qualifierQLocal    = "qlocal"
	qualifierQAlias    = "qalias"
	qualifierQRemote   = "qremote"
	qualifierTopic     = "topic"
	qualifierChannel   = "channel"
)

// Config holds connection parameters for mqweb.
type Config struct {
	Endpoint     *url.URL
	RESTPrefix   string
	QueueManager string
	Username     string
	Password     string
	TLSConfig    *tls.Config
	HTTPClient   *http.Client
	Resilience   ResilienceConfig
}

// Client implements mqadmin.Admin over the mqweb /mqsc endpoint.
type Client struct {
	mqscURL      string
	adminQMURL   string
	queueManager string
	httpClient   *http.Client
	username     string
	password     string
	retry        retryPolicy
	breaker      *circuitBreaker
}

// NewClient builds an mqrest client from Config.
func NewClient(cfg Config) (*Client, error) {
	if cfg.Endpoint == nil {
		return nil, fmt.Errorf("endpoint is required")
	}
	if cfg.QueueManager == "" {
		return nil, fmt.Errorf("queue manager name is required")
	}
	prefix := cfg.RESTPrefix
	if prefix == "" {
		prefix = DefaultRESTPrefix
	}

	base := strings.TrimSuffix(cfg.Endpoint.String(), "/") + prefix
	qm := url.PathEscape(cfg.QueueManager)
	mqscURL := fmt.Sprintf("%s/admin/action/qmgr/%s/mqsc", base, qm)
	adminQMURL := fmt.Sprintf("%s/admin/qmgr/%s", base, qm)

	hc := cfg.HTTPClient
	if hc == nil {
		tr := http.DefaultTransport.(*http.Transport).Clone()
		if cfg.TLSConfig != nil {
			tr.TLSClientConfig = cfg.TLSConfig.Clone()
		}
		hc = &http.Client{Timeout: 60 * time.Second, Transport: tr}
	}

	return &Client{
		mqscURL:      mqscURL,
		adminQMURL:   adminQMURL,
		queueManager: cfg.QueueManager,
		httpClient:   hc,
		username:     cfg.Username,
		password:     cfg.Password,
		retry:        retryPolicyFromResilience(cfg.Resilience),
		breaker:      newCircuitBreaker(circuitBreakerConfigFromResilience(cfg.Resilience)),
	}, nil
}

// CloseIdleConnections closes idle connections on the underlying HTTP transport.
func (c *Client) CloseIdleConnections() {
	if c == nil || c.httpClient == nil || c.httpClient.Transport == nil {
		return
	}
	type idleCloser interface {
		CloseIdleConnections()
	}
	if tr, ok := c.httpClient.Transport.(idleCloser); ok {
		tr.CloseIdleConnections()
	}
}

// Ping verifies mqweb can reach the queue manager.
func (c *Client) Ping(ctx context.Context) error {
	var err error
	defer func() { metrics.RecordMQOperation(metrics.MQOpPing, err) }()

	res, err := c.roundTrip(ctx, func(ctx context.Context) (*http.Request, error) {
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, c.adminQMURL, nil)
		if reqErr != nil {
			return nil, reqErr
		}
		req.SetBasicAuth(c.username, c.password)
		return req, nil
	})
	if err != nil {
		return err
	}
	defer closeBody(res.Body)

	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
		return &mqadmin.TerminalError{
			Reason:  "Unauthorized",
			Message: fmt.Sprintf("mqweb ping returned HTTP %d", res.StatusCode),
		}
	}
	if res.StatusCode >= 500 {
		return &mqadmin.TransientError{Message: fmt.Sprintf("mqweb ping returned HTTP %d", res.StatusCode)}
	}
	if res.StatusCode >= 400 {
		return &mqadmin.TerminalError{
			Reason:  "Unreachable",
			Message: fmt.Sprintf("mqweb ping returned HTTP %d", res.StatusCode),
		}
	}
	return nil
}

// GetQueue returns observed attributes for a queue (local, alias, or remote).
func (c *Client) GetQueue(ctx context.Context, spec mqadmin.QueueSpec) (*mqadmin.QueueState, error) {
	var err error
	defer func() { metrics.RecordMQOperation(metrics.MQOpGetQueue, err) }()

	if err = validateQueueType(spec.Type); err != nil {
		return nil, err
	}

	resp, err := c.runCommandJSON(ctx, queueDisplayRequest(spec))
	if err != nil {
		return nil, err
	}
	attrs, err := resp.firstObjectAttributes()
	if err != nil {
		if nf := (*mqadmin.NotFoundError)(nil); errors.As(err, &nf) {
			err = &mqadmin.NotFoundError{Object: spec.Name}
		}
		return nil, err
	}
	normalizeQueueAttributes(attrs, spec.Type)
	return &mqadmin.QueueState{Name: spec.Name, Attributes: attrs}, nil
}

// DefineQueue creates or updates a local queue (REPLACE).
func (c *Client) DefineQueue(ctx context.Context, spec mqadmin.QueueSpec) error {
	var err error
	defer func() { metrics.RecordMQOperation(metrics.MQOpDefineQueue, err) }()

	if err = validateQueueType(spec.Type); err != nil {
		return err
	}
	params := defineQueueParameters(spec)
	_, err = c.runCommandJSON(ctx, runCommandJSONRequest{
		Type:       mqscType,
		Command:    mqscCommandDefine,
		Qualifier:  queueQualifier(spec.Type),
		Name:       spec.Name,
		Parameters: params,
	})
	return err
}

// RunMQSC executes a single MQSC command string via the runCommand API.
func (c *Client) RunMQSC(ctx context.Context, command string) error {
	var err error
	defer func() { metrics.RecordMQOperation(metrics.MQOpRunMQSC, err) }()

	body := runCommandRequest{Type: "runCommand"}
	body.Parameters.Command = command
	_, err = c.postMQSC(ctx, body)
	return err
}

// DeleteQueue removes a queue (local, alias, or remote).
func (c *Client) DeleteQueue(ctx context.Context, spec mqadmin.QueueSpec) error {
	var err error
	defer func() { metrics.RecordMQOperation(metrics.MQOpDeleteQueue, err) }()

	if err = validateQueueType(spec.Type); err != nil {
		return err
	}

	_, err = c.runCommandJSON(ctx, runCommandJSONRequest{
		Type:      mqscType,
		Command:   mqscCommandDelete,
		Qualifier: queueQualifier(spec.Type),
		Name:      spec.Name,
	})
	if err != nil && errors.Is(err, mqadmin.ErrNotFound) {
		err = nil
		return nil
	}
	return err
}

func validateQueueType(qType mqadmin.QueueType) error {
	switch mqadmin.NormalizeQueueType(qType) {
	case mqadmin.QueueTypeLocal, mqadmin.QueueTypeAlias, mqadmin.QueueTypeRemote:
		return nil
	default:
		return &mqadmin.TerminalError{
			Reason:  "UnsupportedQueueType",
			Message: fmt.Sprintf("queue type %q is not supported", qType),
		}
	}
}

// GetTopic returns the observed attributes of a topic.
func (c *Client) GetTopic(ctx context.Context, name string) (*mqadmin.TopicState, error) {
	var err error
	defer func() { metrics.RecordMQOperation(metrics.MQOpGetTopic, err) }()

	resp, err := c.runCommandJSON(ctx, runCommandJSONRequest{
		Type:               mqscType,
		Command:            mqscCommandDisplay,
		Qualifier:          qualifierTopic,
		Name:               name,
		ResponseParameters: append([]string(nil), topicDisplayParameters...),
	})
	if err != nil {
		return nil, err
	}
	attrs, err := resp.firstObjectAttributes()
	if err != nil {
		if nf := (*mqadmin.NotFoundError)(nil); errors.As(err, &nf) {
			err = &mqadmin.NotFoundError{Object: name}
		}
		return nil, err
	}
	normalizeTopicAttributes(attrs)
	return &mqadmin.TopicState{Name: name, Attributes: attrs}, nil
}

// DefineTopic creates or updates a topic.
func (c *Client) DefineTopic(ctx context.Context, spec mqadmin.TopicSpec) error {
	var err error
	defer func() { metrics.RecordMQOperation(metrics.MQOpDefineTopic, err) }()

	_, err = c.runCommandJSON(ctx, runCommandJSONRequest{
		Type:       mqscType,
		Command:    mqscCommandDefine,
		Qualifier:  qualifierTopic,
		Name:       spec.Name,
		Parameters: defineTopicParameters(spec),
	})
	return err
}

// DeleteTopic removes a topic.
func (c *Client) DeleteTopic(ctx context.Context, name string) error {
	var err error
	defer func() { metrics.RecordMQOperation(metrics.MQOpDeleteTopic, err) }()

	_, err = c.runCommandJSON(ctx, runCommandJSONRequest{
		Type:      mqscType,
		Command:   mqscCommandDelete,
		Qualifier: qualifierTopic,
		Name:      name,
	})
	if err != nil && errors.Is(err, mqadmin.ErrNotFound) {
		err = nil
		return nil
	}
	return err
}

// GetChannel returns the observed attributes of a channel.
func (c *Client) GetChannel(ctx context.Context, spec mqadmin.ChannelSpec) (*mqadmin.ChannelState, error) {
	var err error
	defer func() { metrics.RecordMQOperation(metrics.MQOpGetChannel, err) }()

	resp, err := c.runCommandJSON(ctx, channelDisplayRequest(spec.Name, spec.Type))
	if err != nil {
		return nil, err
	}
	attrs, err := resp.firstObjectAttributes()
	if err != nil {
		if nf := (*mqadmin.NotFoundError)(nil); errors.As(err, &nf) {
			err = &mqadmin.NotFoundError{Object: spec.Name}
		}
		return nil, err
	}
	return &mqadmin.ChannelState{Name: spec.Name, Attributes: attrs}, nil
}

// DefineChannel creates or updates a channel.
func (c *Client) DefineChannel(ctx context.Context, spec mqadmin.ChannelSpec) error {
	var err error
	defer func() { metrics.RecordMQOperation(metrics.MQOpDefineChannel, err) }()

	_, err = c.runCommandJSON(ctx, runCommandJSONRequest{
		Type:       mqscType,
		Command:    mqscCommandDefine,
		Qualifier:  qualifierChannel,
		Name:       spec.Name,
		Parameters: defineChannelParameters(spec),
	})
	return err
}

// DeleteChannel removes a channel.
func (c *Client) DeleteChannel(ctx context.Context, spec mqadmin.ChannelSpec) error {
	var err error
	defer func() { metrics.RecordMQOperation(metrics.MQOpDeleteChannel, err) }()

	// mqweb rejects chltype on DELETE; omit parameters (AMQ8147E still maps to ErrNotFound).
	_, err = c.runCommandJSON(ctx, runCommandJSONRequest{
		Type:      mqscType,
		Command:   mqscCommandDelete,
		Qualifier: qualifierChannel,
		Name:      spec.Name,
	})
	if err != nil && errors.Is(err, mqadmin.ErrNotFound) {
		err = nil
		return nil
	}
	return err
}

func (c *Client) runCommandJSON(ctx context.Context, body runCommandJSONRequest) (*mqscResponse, error) {
	return c.postMQSC(ctx, body)
}

func (c *Client) postMQSC(ctx context.Context, body any) (*mqscResponse, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal mqsc request: %w", err)
	}

	res, err := c.roundTrip(ctx, func(ctx context.Context) (*http.Request, error) {
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, c.mqscURL, bytes.NewReader(payload))
		if reqErr != nil {
			return nil, reqErr
		}
		req.Header.Set("Content-Type", "application/json; charset=UTF-8")
		req.Header.Set(csrfHeader, "1")
		req.SetBasicAuth(c.username, c.password)
		return req, nil
	})
	if err != nil {
		return nil, err
	}
	defer closeBody(res.Body)

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, &mqadmin.TransientError{Message: "read mqweb response", Cause: err}
	}

	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
		return nil, &mqadmin.TerminalError{
			Reason:  "Unauthorized",
			Message: fmt.Sprintf("mqweb returned HTTP %d", res.StatusCode),
		}
	}
	if res.StatusCode >= 500 || res.StatusCode == http.StatusServiceUnavailable {
		return nil, &mqadmin.TransientError{
			Message: fmt.Sprintf("mqweb returned HTTP %d", res.StatusCode),
		}
	}
	if res.StatusCode >= 400 {
		return nil, &mqadmin.TerminalError{
			Reason:  "BadRequest",
			Message: fmt.Sprintf("mqweb returned HTTP %d: %s", res.StatusCode, truncate(string(raw), 200)),
		}
	}

	var parsed mqscResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, &mqadmin.TerminalError{
			Reason:  "InvalidResponse",
			Message: "failed to parse mqsc response",
			Cause:   err,
		}
	}
	if parsed.overallFailed() {
		if parsed.isObjectMissing() {
			obj := ""
			if req, ok := body.(runCommandJSONRequest); ok {
				obj = req.Name
			}
			return &parsed, &mqadmin.NotFoundError{Object: obj}
		}
		return &parsed, parsed.terminalError("mqsc command failed")
	}
	return &parsed, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func closeBody(body io.ReadCloser) {
	_ = body.Close()
}
