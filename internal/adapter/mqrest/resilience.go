package mqrest

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/conduit-ops/mkurator/internal/mqadmin"
)

const (
	defaultMaxAttempts      = 4
	defaultInitialBackoff   = 200 * time.Millisecond
	defaultMaxBackoff       = 5 * time.Second
	defaultFailureThreshold = 5
	defaultOpenTimeout      = 30 * time.Second
)

// ResilienceConfig tunes mqweb retry and per-connection circuit breaking.
type ResilienceConfig struct {
	MaxAttempts      int
	InitialBackoff   time.Duration
	MaxBackoff       time.Duration
	FailureThreshold int
	OpenTimeout      time.Duration
}

type retryPolicy struct {
	maxAttempts    int
	initialBackoff time.Duration
	maxBackoff     time.Duration
	sleep          func(time.Duration)
}

func defaultRetryPolicy() retryPolicy {
	return retryPolicyFromResilience(ResilienceConfig{})
}

func retryPolicyFromResilience(cfg ResilienceConfig) retryPolicy {
	p := retryPolicy{
		maxAttempts:    defaultMaxAttempts,
		initialBackoff: defaultInitialBackoff,
		maxBackoff:     defaultMaxBackoff,
		sleep:          time.Sleep,
	}
	if cfg.MaxAttempts > 0 {
		p.maxAttempts = cfg.MaxAttempts
	}
	if cfg.InitialBackoff > 0 {
		p.initialBackoff = cfg.InitialBackoff
	}
	if cfg.MaxBackoff > 0 {
		p.maxBackoff = cfg.MaxBackoff
	}
	return p
}

func (p retryPolicy) backoff(attempt int) time.Duration {
	if attempt <= 0 {
		return p.initialBackoff
	}
	d := p.initialBackoff
	for i := 1; i < attempt; i++ {
		d *= 2
		if d >= p.maxBackoff {
			return p.maxBackoff
		}
	}
	return d
}

func isRetryableHTTPStatus(code int) bool {
	return code == http.StatusTooManyRequests || code >= http.StatusInternalServerError
}

type requestBuilder func(context.Context) (*http.Request, error)

func (c *Client) roundTrip(ctx context.Context, build requestBuilder) (*http.Response, error) {
	if err := c.breaker.beforeRequest(); err != nil {
		return nil, err
	}

	var lastNetErr error
	for attempt := 0; attempt < c.retry.maxAttempts; attempt++ {
		if attempt > 0 {
			if err := sleepWithContext(ctx, c.retry.sleep, c.retry.backoff(attempt)); err != nil {
				return nil, err
			}
		}

		req, err := build(ctx)
		if err != nil {
			return nil, fmt.Errorf("build mqweb request: %w", err)
		}

		res, err := c.httpClient.Do(req)
		if err != nil {
			lastNetErr = err
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			continue
		}

		if isRetryableHTTPStatus(res.StatusCode) {
			drainAndClose(res.Body)
			if attempt+1 < c.retry.maxAttempts {
				continue
			}
			c.breaker.recordFailure()
			return nil, &mqadmin.TransientError{
				Message: fmt.Sprintf("mqweb returned HTTP %d", res.StatusCode),
			}
		}

		c.breaker.recordSuccess()
		return res, nil
	}

	c.breaker.recordFailure()
	if lastNetErr != nil {
		return nil, &mqadmin.TransientError{Message: "mqweb request failed", Cause: lastNetErr}
	}
	return nil, &mqadmin.TransientError{Message: "mqweb request failed after retries"}
}

func sleepWithContext(ctx context.Context, sleepFn func(time.Duration), d time.Duration) error {
	if sleepFn == nil {
		sleepFn = time.Sleep
	}
	done := make(chan struct{})
	go func() {
		sleepFn(d)
		close(done)
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return nil
	}
}

func drainAndClose(body io.ReadCloser) {
	if body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, body)
	closeBody(body)
}
