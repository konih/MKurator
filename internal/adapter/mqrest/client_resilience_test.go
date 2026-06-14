package mqrest_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/conduit-ops/mkurator/internal/adapter/mqrest"
)

func TestClient_PingRetriesServerError(t *testing.T) {
	t.Parallel()
	var attempts atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) < 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	c, err := mqrest.NewClient(
		mqrest.Config{
			Endpoint:     u,
			QueueManager: "QM1",
			Username:     "admin",
			Password:     "pass",
			HTTPClient:   srv.Client(),
			Resilience: mqrest.ResilienceConfig{
				MaxAttempts:    3,
				InitialBackoff: time.Millisecond,
				MaxBackoff:     5 * time.Millisecond,
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	if attempts.Load() != 2 {
		t.Fatalf("attempts=%d", attempts.Load())
	}
}
