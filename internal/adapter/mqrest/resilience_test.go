package mqrest

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/conduit-ops/mkurator/internal/mqadmin"
)

func TestDefaultRetryPolicy(t *testing.T) {
	t.Parallel()
	p := defaultRetryPolicy()
	if p.maxAttempts != defaultMaxAttempts || p.initialBackoff != defaultInitialBackoff {
		t.Fatalf("policy=%+v", p)
	}
}

func TestRetryPolicyBackoff(t *testing.T) {
	t.Parallel()
	p := retryPolicy{initialBackoff: 100 * time.Millisecond, maxBackoff: 500 * time.Millisecond}
	if p.backoff(0) != 100*time.Millisecond {
		t.Fatal("attempt 0 should return initial backoff")
	}
	if p.backoff(4) != 500*time.Millisecond {
		t.Fatalf("backoff(4)=%v", p.backoff(4))
	}
}

func TestRetryPolicyFromResilienceOverrides(t *testing.T) {
	t.Parallel()
	p := retryPolicyFromResilience(ResilienceConfig{
		MaxAttempts:    2,
		InitialBackoff: time.Second,
		MaxBackoff:     2 * time.Second,
	})
	if p.maxAttempts != 2 || p.initialBackoff != time.Second || p.maxBackoff != 2*time.Second {
		t.Fatalf("policy=%+v", p)
	}
}

func TestRoundTripRetriesTransientHTTPStatus(t *testing.T) {
	t.Parallel()
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	c := &Client{
		httpClient: srv.Client(),
		retry: retryPolicy{
			maxAttempts:    4,
			initialBackoff: time.Millisecond,
			maxBackoff:     5 * time.Millisecond,
			sleep:          time.Sleep,
		},
		breaker: newCircuitBreaker(defaultCircuitBreakerConfig()),
	}
	res, err := c.roundTrip(context.Background(), func(ctx context.Context) (*http.Request, error) {
		return http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	})
	if err != nil {
		t.Fatal(err)
	}
	defer closeBody(res.Body)
	if attempts.Load() != 3 {
		t.Fatalf("attempts=%d", attempts.Load())
	}
}

func TestRoundTripOpenCircuitSkipsHTTP(t *testing.T) {
	t.Parallel()
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	c := &Client{
		httpClient: srv.Client(),
		retry: retryPolicy{
			maxAttempts:    1,
			initialBackoff: time.Millisecond,
			maxBackoff:     time.Millisecond,
			sleep:          time.Sleep,
		},
		breaker: newCircuitBreaker(
			circuitBreakerConfig{failureThreshold: 1, openTimeout: time.Minute, now: time.Now},
		),
	}
	_, _ = c.roundTrip(context.Background(), func(ctx context.Context) (*http.Request, error) {
		return http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	})
	_, err := c.roundTrip(context.Background(), func(ctx context.Context) (*http.Request, error) {
		return http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	})
	if err == nil || !errors.Is(err, mqadmin.ErrTransient) {
		t.Fatalf("got %v", err)
	}
	if attempts.Load() != 1 {
		t.Fatalf("attempts=%d", attempts.Load())
	}
}

func TestIsRetryableHTTPStatus(t *testing.T) {
	t.Parallel()
	if !isRetryableHTTPStatus(http.StatusTooManyRequests) || isRetryableHTTPStatus(http.StatusBadRequest) {
		t.Fatal("unexpected retry classification")
	}
	if !isRetryableHTTPStatus(http.StatusInternalServerError) {
		t.Fatal("expected 5xx to be retryable")
	}
}

func TestRoundTripBuildError(t *testing.T) {
	t.Parallel()
	c := &Client{
		retry:   defaultRetryPolicy(),
		breaker: newCircuitBreaker(defaultCircuitBreakerConfig()),
	}
	_, err := c.roundTrip(context.Background(), func(context.Context) (*http.Request, error) {
		return nil, errors.New("build failed")
	})
	if err == nil || !strings.Contains(err.Error(), "build mqweb request") {
		t.Fatalf("got %v", err)
	}
}

func TestRoundTripContextCancelledDuringBackoff(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	c := &Client{
		httpClient: srv.Client(),
		retry: retryPolicy{
			maxAttempts:    3,
			initialBackoff: time.Hour,
			maxBackoff:     time.Hour,
			sleep:          time.Sleep,
		},
		breaker: newCircuitBreaker(defaultCircuitBreakerConfig()),
	}
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	_, err := c.roundTrip(ctx, func(reqCtx context.Context) (*http.Request, error) {
		return http.NewRequestWithContext(reqCtx, http.MethodGet, srv.URL, nil)
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}

func TestRoundTripExhaustedHTTPRetries(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	c := &Client{
		httpClient: srv.Client(),
		retry: retryPolicy{
			maxAttempts:    2,
			initialBackoff: time.Millisecond,
			maxBackoff:     time.Millisecond,
			sleep:          time.Sleep,
		},
		breaker: newCircuitBreaker(defaultCircuitBreakerConfig()),
	}
	_, err := c.roundTrip(context.Background(), func(ctx context.Context) (*http.Request, error) {
		return http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	})
	if err == nil || !errors.Is(err, mqadmin.ErrTransient) {
		t.Fatalf("got %v", err)
	}
}

func TestRoundTripExhaustedNetworkRetries(t *testing.T) {
	t.Parallel()
	c := &Client{
		httpClient: &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("connection refused")
		})},
		retry: retryPolicy{
			maxAttempts:    2,
			initialBackoff: time.Millisecond,
			maxBackoff:     time.Millisecond,
			sleep:          time.Sleep,
		},
		breaker: newCircuitBreaker(defaultCircuitBreakerConfig()),
	}
	_, err := c.roundTrip(context.Background(), func(ctx context.Context) (*http.Request, error) {
		return http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:1", nil)
	})
	if err == nil || !errors.Is(err, mqadmin.ErrTransient) {
		t.Fatalf("got %v", err)
	}
}

func TestRoundTripContextCancelledDuringRequest(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c := &Client{
		httpClient: &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("connection reset")
		})},
		retry: retryPolicy{
			maxAttempts:    3,
			initialBackoff: time.Millisecond,
			maxBackoff:     time.Millisecond,
			sleep:          time.Sleep,
		},
		breaker: newCircuitBreaker(defaultCircuitBreakerConfig()),
	}
	_, err := c.roundTrip(ctx, func(reqCtx context.Context) (*http.Request, error) {
		return http.NewRequestWithContext(reqCtx, http.MethodGet, "http://127.0.0.1:1", nil)
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}

func TestSleepWithContextCancelled(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := sleepWithContext(ctx, func(time.Duration) {}, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}

func TestSleepWithContextNilSleepFn(t *testing.T) {
	t.Parallel()
	if err := sleepWithContext(context.Background(), nil, time.Nanosecond); err != nil {
		t.Fatal(err)
	}
}

func TestDrainAndCloseNil(t *testing.T) {
	t.Parallel()
	drainAndClose(nil)
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestRoundTripNetworkErrorRetries(t *testing.T) {
	t.Parallel()
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) == 1 {
			hj, _ := w.(http.Hijacker)
			conn, _, _ := hj.Hijack()
			_ = conn.Close()
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	}))
	defer srv.Close()
	c := &Client{
		httpClient: srv.Client(),
		retry: retryPolicy{
			maxAttempts:    3,
			initialBackoff: time.Millisecond,
			maxBackoff:     5 * time.Millisecond,
			sleep:          time.Sleep,
		},
		breaker: newCircuitBreaker(defaultCircuitBreakerConfig()),
	}
	res, err := c.roundTrip(context.Background(), func(ctx context.Context) (*http.Request, error) {
		return http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	})
	if err != nil {
		t.Fatal(err)
	}
	defer closeBody(res.Body)
	if attempts.Load() != 2 {
		t.Fatalf("attempts=%d", attempts.Load())
	}
}
