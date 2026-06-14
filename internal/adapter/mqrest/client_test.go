package mqrest_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/conduit-ops/mkurator/internal/adapter/mqrest"
	"github.com/conduit-ops/mkurator/internal/mqadmin"
)

const (
	testKeyCommandResponse       = "commandResponse"
	testKeyCompletionCode        = "completionCode"
	testKeyOverallCompletionCode = "overallCompletionCode"
)

func TestClient_PingSuccess(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ibmmq/rest/v3/admin/qmgr/QM1" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, srv.Client())
	if err := c.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestClient_DefineAndGetQueue(t *testing.T) {
	t.Parallel()
	var lastBody map[string]any
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&lastBody); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if lastBody["command"] == "display" {
			rp, _ := lastBody["responseParameters"].([]any)
			for _, p := range rp {
				if p == "maxmsglen" {
					t.Error("display must not request maxmsglen")
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				testKeyCommandResponse: []map[string]any{{
					testKeyCompletionCode: 0,
					"parameters":          map[string]any{"maxdepth": "5000", "descr": "orders"},
				}},
				testKeyOverallCompletionCode: 0,
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			testKeyCommandResponse:       []map[string]any{{testKeyCompletionCode: 0}},
			testKeyOverallCompletionCode: 0,
		})
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, srv.Client())
	spec := mqadmin.QueueSpec{
		Name: "APP.ORDERS",
		Type: mqadmin.QueueTypeLocal,
		Attributes: map[string]string{
			"maxdepth": "5000",
			"descr":    "orders",
		},
	}
	if err := c.DefineQueue(context.Background(), spec); err != nil {
		t.Fatalf("DefineQueue: %v", err)
	}
	if lastBody["type"] != "runCommandJSON" {
		t.Fatalf("define type = %v", lastBody["type"])
	}
	params, _ := lastBody["parameters"].(map[string]any)
	if params["maxdepth"] != float64(5000) && params["maxdepth"] != 5000 {
		t.Fatalf("maxdepth param = %T(%v)", params["maxdepth"], params["maxdepth"])
	}
	state, err := c.GetQueue(context.Background(), spec)
	if err != nil {
		t.Fatalf("GetQueue: %v", err)
	}
	if state.Attributes["maxdepth"] != "5000" {
		t.Fatalf("maxdepth = %q", state.Attributes["maxdepth"])
	}
}

func TestClient_GetQueueNotFound(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			testKeyCommandResponse: []map[string]any{{
				testKeyCompletionCode: 2,
				"message":             []string{"AMQ8147E: IBM MQ object APP.MISSING not found."},
			}},
			testKeyOverallCompletionCode: 2,
		})
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, srv.Client())
	_, err := c.GetQueue(context.Background(), mqadmin.QueueSpec{Name: "APP.MISSING", Type: mqadmin.QueueTypeLocal})
	if err == nil {
		t.Fatal("expected not found error")
	}
	if !errors.Is(err, mqadmin.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestClient_RunMQSC(t *testing.T) {
	t.Parallel()
	var lastBody map[string]any
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&lastBody); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			testKeyOverallCompletionCode: 0,
			testKeyCommandResponse:       []map[string]any{{testKeyCompletionCode: 0}},
		})
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, srv.Client())
	cmd := "DEFINE CHANNEL('APP.CH') CHLTYPE(SVRCONN) REPLACE"
	if err := c.RunMQSC(context.Background(), cmd); err != nil {
		t.Fatalf("RunMQSC: %v", err)
	}
	if lastBody["type"] != "runCommand" {
		t.Fatalf("type = %v", lastBody["type"])
	}
	params, _ := lastBody["parameters"].(map[string]any)
	if params["command"] != cmd {
		t.Fatalf("command = %v", params["command"])
	}
}

func newTestClient(t *testing.T, endpoint string, hc *http.Client) *mqrest.Client {
	t.Helper()
	u, err := url.Parse(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	c, err := mqrest.NewClient(mqrest.Config{
		Endpoint:     u,
		QueueManager: "QM1",
		Username:     "admin",
		Password:     "pass",
		HTTPClient:   hc,
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestNewClientValidation(t *testing.T) {
	t.Parallel()
	_, err := mqrest.NewClient(mqrest.Config{})
	if err == nil {
		t.Fatal("expected error for missing endpoint")
	}
	u, _ := url.Parse("https://mq.example:9443")
	_, err = mqrest.NewClient(mqrest.Config{Endpoint: u})
	if err == nil {
		t.Fatal("expected error for missing queue manager")
	}
}

func TestClient_PingUnauthorized(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL, srv.Client())
	err := c.Ping(context.Background())
	if !errors.Is(err, mqadmin.ErrTerminal) {
		t.Fatalf("expected terminal error, got %v", err)
	}
}

func TestClient_DeleteQueue(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			testKeyOverallCompletionCode: 0,
			testKeyCommandResponse:       []map[string]any{{testKeyCompletionCode: 0}},
		})
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL, srv.Client())
	spec := mqadmin.QueueSpec{Name: "APP.ORDERS", Type: mqadmin.QueueTypeLocal}
	if err := c.DeleteQueue(context.Background(), spec); err != nil {
		t.Fatalf("DeleteQueue: %v", err)
	}
}

func TestClient_DeleteQueueNotFound(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			testKeyCommandResponse: []map[string]any{{
				testKeyCompletionCode: 2,
				"message":             []string{"AMQ8147E: IBM MQ object APP.GONE not found."},
			}},
			testKeyOverallCompletionCode: 2,
		})
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL, srv.Client())
	spec := mqadmin.QueueSpec{Name: "APP.GONE", Type: mqadmin.QueueTypeLocal}
	if err := c.DeleteQueue(context.Background(), spec); err != nil {
		t.Fatalf("DeleteQueue not found should succeed: %v", err)
	}
}

func TestClient_DefineQueueUnsupportedType(t *testing.T) {
	t.Parallel()
	u, _ := url.Parse("https://mq.example:9443")
	c, err := mqrest.NewClient(mqrest.Config{
		Endpoint: u, QueueManager: "QM1", Username: "a", Password: "b",
	})
	if err != nil {
		t.Fatal(err)
	}
	err = c.DefineQueue(context.Background(), mqadmin.QueueSpec{
		Name: "X", Type: mqadmin.QueueType("model"),
	})
	if !errors.Is(err, mqadmin.ErrTerminal) {
		t.Fatalf("expected terminal error, got %v", err)
	}
}

func TestClient_PostMQSCServerError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL, srv.Client())
	err := c.RunMQSC(context.Background(), "DISPLAY QMGR")
	if !errors.Is(err, mqadmin.ErrTransient) {
		t.Fatalf("expected transient error, got %v", err)
	}
}

func TestClient_PostMQSCBadRequestLongBody(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(strings.Repeat("x", 300)))
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL, srv.Client())
	err := c.RunMQSC(context.Background(), "DISPLAY QMGR")
	if !errors.Is(err, mqadmin.ErrTerminal) {
		t.Fatalf("expected terminal error, got %v", err)
	}
}

func TestClient_PostMQSCInvalidJSON(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not-json"))
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL, srv.Client())
	err := c.RunMQSC(context.Background(), "DISPLAY QMGR")
	if !errors.Is(err, mqadmin.ErrTerminal) {
		t.Fatalf("expected terminal error, got %v", err)
	}
}

func TestClient_PingNotFound(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL, srv.Client())
	err := c.Ping(context.Background())
	if !errors.Is(err, mqadmin.ErrTerminal) {
		t.Fatalf("expected terminal error, got %v", err)
	}
}

func TestClient_PingForbidden(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL, srv.Client())
	err := c.Ping(context.Background())
	if !errors.Is(err, mqadmin.ErrTerminal) {
		t.Fatalf("expected terminal error, got %v", err)
	}
}

func TestClient_PingNetworkError(t *testing.T) {
	t.Parallel()
	u, _ := url.Parse("https://127.0.0.1:1")
	c, err := mqrest.NewClient(mqrest.Config{
		Endpoint:     u,
		QueueManager: "QM1",
		Username:     "admin",
		Password:     "pass",
		HTTPClient:   &http.Client{Timeout: 10 * time.Millisecond},
	})
	if err != nil {
		t.Fatal(err)
	}
	err = c.Ping(context.Background())
	if !errors.Is(err, mqadmin.ErrTransient) {
		t.Fatalf("expected transient error, got %v", err)
	}
}

func TestClient_PostMQSCForbidden(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL, srv.Client())
	err := c.RunMQSC(context.Background(), "DISPLAY QMGR")
	if !errors.Is(err, mqadmin.ErrTerminal) {
		t.Fatalf("expected terminal error, got %v", err)
	}
}

func TestClient_PingServerError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL, srv.Client())
	err := c.Ping(context.Background())
	if !errors.Is(err, mqadmin.ErrTransient) {
		t.Fatalf("expected transient error, got %v", err)
	}
}

func TestClient_DefineAndGetTopic(t *testing.T) {
	t.Parallel()
	var lastBody map[string]any
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&lastBody); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if lastBody["command"] == "display" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				testKeyCommandResponse: []map[string]any{{
					testKeyCompletionCode: 0,
					"parameters":          map[string]any{"topstr": "retail/orders"},
				}},
				testKeyOverallCompletionCode: 0,
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			testKeyCommandResponse:       []map[string]any{{testKeyCompletionCode: 0}},
			testKeyOverallCompletionCode: 0,
		})
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, srv.Client())
	spec := mqadmin.TopicSpec{
		Name: "RETAIL.ORDERS",
		Attributes: map[string]string{
			"topstr": "retail/orders",
		},
	}
	if err := c.DefineTopic(context.Background(), spec); err != nil {
		t.Fatalf("DefineTopic: %v", err)
	}
	if lastBody["qualifier"] != "topic" {
		t.Fatalf("qualifier = %v", lastBody["qualifier"])
	}
	state, err := c.GetTopic(context.Background(), "RETAIL.ORDERS")
	if err != nil {
		t.Fatalf("GetTopic: %v", err)
	}
	if state.Attributes["topstr"] != "retail/orders" {
		t.Fatalf("topstr = %q", state.Attributes["topstr"])
	}
}

func TestClient_DefineAndGetChannel(t *testing.T) {
	t.Parallel()
	var lastBody map[string]any
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&lastBody); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if lastBody["command"] == "display" {
			params, _ := lastBody["parameters"].(map[string]any)
			if params["chltype"] != "svrconn" {
				t.Errorf("display chltype = %v", params["chltype"])
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				testKeyCommandResponse: []map[string]any{{
					testKeyCompletionCode: 0,
					"parameters":          map[string]any{"trptype": "tcp"},
				}},
				testKeyOverallCompletionCode: 0,
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			testKeyCommandResponse:       []map[string]any{{testKeyCompletionCode: 0}},
			testKeyOverallCompletionCode: 0,
		})
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, srv.Client())
	spec := mqadmin.ChannelSpec{
		Name: "ORDERS.APP",
		Type: mqadmin.ChannelTypeSvrconn,
		Attributes: map[string]string{
			"trptype": "tcp",
			"maxmsgl": "4194304",
		},
	}
	if err := c.DefineChannel(context.Background(), spec); err != nil {
		t.Fatalf("DefineChannel: %v", err)
	}
	if lastBody["qualifier"] != "channel" {
		t.Fatalf("qualifier = %v", lastBody["qualifier"])
	}
	params, _ := lastBody["parameters"].(map[string]any)
	if params["chltype"] != "svrconn" {
		t.Fatalf("define chltype = %v", params["chltype"])
	}
	state, err := c.GetChannel(context.Background(), spec)
	if err != nil {
		t.Fatalf("GetChannel: %v", err)
	}
	if state.Attributes["trptype"] != "tcp" {
		t.Fatalf("trptype = %q", state.Attributes["trptype"])
	}
}

func TestClient_DeleteTopic(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			testKeyOverallCompletionCode: 0,
			testKeyCommandResponse:       []map[string]any{{testKeyCompletionCode: 0}},
		})
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL, srv.Client())
	if err := c.DeleteTopic(context.Background(), "RETAIL.ORDERS"); err != nil {
		t.Fatalf("DeleteTopic: %v", err)
	}
}

func TestClient_DeleteTopicNotFound(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			testKeyCommandResponse: []map[string]any{{
				testKeyCompletionCode: 2,
				"message":             []string{"AMQ8147E: IBM MQ object RETAIL.ORDERS not found."},
			}},
			testKeyOverallCompletionCode: 2,
		})
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL, srv.Client())
	if err := c.DeleteTopic(context.Background(), "RETAIL.ORDERS"); err != nil {
		t.Fatalf("DeleteTopic not found should succeed: %v", err)
	}
}

func TestClient_DeleteChannel(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			testKeyOverallCompletionCode: 0,
			testKeyCommandResponse:       []map[string]any{{testKeyCompletionCode: 0}},
		})
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL, srv.Client())
	spec := mqadmin.ChannelSpec{Name: "ORDERS.APP", Type: mqadmin.ChannelTypeSvrconn}
	if err := c.DeleteChannel(context.Background(), spec); err != nil {
		t.Fatalf("DeleteChannel: %v", err)
	}
}

func TestClient_DeleteChannelNotFound(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			testKeyCommandResponse: []map[string]any{{
				testKeyCompletionCode: 2,
				"message":             []string{"AMQ8147E: IBM MQ object ORDERS.APP not found."},
			}},
			testKeyOverallCompletionCode: 2,
		})
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL, srv.Client())
	spec := mqadmin.ChannelSpec{Name: "ORDERS.APP", Type: mqadmin.ChannelTypeSvrconn}
	if err := c.DeleteChannel(context.Background(), spec); err != nil {
		t.Fatalf("DeleteChannel not found should succeed: %v", err)
	}
}

func TestClient_GetTopicNotFound(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			testKeyCommandResponse: []map[string]any{{
				testKeyCompletionCode: 2,
				"message":             []string{"AMQ8147E: IBM MQ object RETAIL.MISSING not found."},
			}},
			testKeyOverallCompletionCode: 2,
		})
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL, srv.Client())
	_, err := c.GetTopic(context.Background(), "RETAIL.MISSING")
	if err == nil {
		t.Fatal("expected not found error")
	}
	if !errors.Is(err, mqadmin.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestClient_GetChannelNotFound(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			testKeyCommandResponse: []map[string]any{{
				testKeyCompletionCode: 2,
				"message":             []string{"AMQ8147E: IBM MQ object ORDERS.MISSING not found."},
			}},
			testKeyOverallCompletionCode: 2,
		})
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL, srv.Client())
	spec := mqadmin.ChannelSpec{Name: "ORDERS.MISSING", Type: mqadmin.ChannelTypeSvrconn}
	_, err := c.GetChannel(context.Background(), spec)
	if err == nil {
		t.Fatal("expected not found error")
	}
	if !errors.Is(err, mqadmin.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestClient_DefineAndGetAliasQueue(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			return
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if body["command"] == "display" {
			if body["qualifier"] != "qalias" {
				t.Errorf("qualifier = %v", body["qualifier"])
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				testKeyCommandResponse: []map[string]any{{
					testKeyCompletionCode: 0,
					"parameters":          map[string]any{"target": "APP.BASE", "descr": "alias"},
				}},
				testKeyOverallCompletionCode: 0,
			})
			return
		}
		if body["qualifier"] != "qalias" {
			t.Errorf("define qualifier = %v", body["qualifier"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			testKeyCommandResponse:       []map[string]any{{testKeyCompletionCode: 0}},
			testKeyOverallCompletionCode: 0,
		})
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, srv.Client())
	spec := mqadmin.QueueSpec{
		Name: "APP.ALIAS",
		Type: mqadmin.QueueTypeAlias,
		Attributes: map[string]string{
			"targq": "APP.BASE",
			"descr": "alias",
		},
	}
	if err := c.DefineQueue(context.Background(), spec); err != nil {
		t.Fatalf("DefineQueue: %v", err)
	}
	state, err := c.GetQueue(context.Background(), spec)
	if err != nil {
		t.Fatalf("GetQueue: %v", err)
	}
	if state.Attributes["targq"] != "APP.BASE" {
		t.Fatalf("targq = %q", state.Attributes["targq"])
	}
}

func TestClient_DefineAndGetRemoteQueue(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			return
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if body["command"] == "display" {
			if body["qualifier"] != "qremote" {
				t.Errorf("qualifier = %v", body["qualifier"])
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				testKeyCommandResponse: []map[string]any{{
					testKeyCompletionCode: 0,
					"parameters": map[string]any{
						"remotequeue":       "REMOTE.Q",
						"remotemanager":     "QM2",
						"transmissionqueue": "XMIT.Q",
					},
				}},
				testKeyOverallCompletionCode: 0,
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			testKeyCommandResponse:       []map[string]any{{testKeyCompletionCode: 0}},
			testKeyOverallCompletionCode: 0,
		})
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, srv.Client())
	spec := mqadmin.QueueSpec{
		Name: "APP.REMOTE",
		Type: mqadmin.QueueTypeRemote,
		Attributes: map[string]string{
			"rname":   "REMOTE.Q",
			"rqmname": "QM2",
			"xmitq":   "XMIT.Q",
		},
	}
	if err := c.DefineQueue(context.Background(), spec); err != nil {
		t.Fatalf("DefineQueue: %v", err)
	}
	state, err := c.GetQueue(context.Background(), spec)
	if err != nil {
		t.Fatalf("GetQueue: %v", err)
	}
	if state.Attributes["rname"] != "REMOTE.Q" || state.Attributes["rqmname"] != "QM2" {
		t.Fatalf("attrs = %v", state.Attributes)
	}
}

func TestClient_SetAndDeleteChannelAuth(t *testing.T) {
	t.Parallel()
	var lastCmd string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		params, _ := body["parameters"].(map[string]any)
		lastCmd, _ = params["command"].(string)
		_ = json.NewEncoder(w).Encode(map[string]any{
			testKeyOverallCompletionCode: 0,
			testKeyCommandResponse:       []map[string]any{{testKeyCompletionCode: 0}},
		})
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, srv.Client())
	spec := mqadmin.ChannelAuthSpec{
		ChannelName: "DEV.APP.SVRCONN.0TLS",
		RuleType:    mqadmin.ChannelAuthRuleTypeAddressMap,
		Address:     "*",
		UserSource:  "CHANNEL",
		CheckClient: "REQUIRED",
	}
	if err := c.SetChannelAuth(context.Background(), spec); err != nil {
		t.Fatalf("SetChannelAuth: %v", err)
	}
	if !strings.Contains(lastCmd, "ACTION(REPLACE)") {
		t.Fatalf("command = %q", lastCmd)
	}
	if err := c.DeleteChannelAuth(context.Background(), spec); err != nil {
		t.Fatalf("DeleteChannelAuth: %v", err)
	}
	wantRemove := "SET CHLAUTH('DEV.APP.SVRCONN.0TLS') TYPE(ADDRESSMAP) ADDRESS('*') ACTION(REMOVE)"
	if lastCmd != wantRemove {
		t.Fatalf("command = %q, want %q", lastCmd, wantRemove)
	}
}

func TestClient_SetAndDeleteAuthority(t *testing.T) {
	t.Parallel()
	var lastCmd string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		params, _ := body["parameters"].(map[string]any)
		lastCmd, _ = params["command"].(string)
		_ = json.NewEncoder(w).Encode(map[string]any{
			testKeyOverallCompletionCode: 0,
			testKeyCommandResponse:       []map[string]any{{testKeyCompletionCode: 0}},
		})
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, srv.Client())
	spec := mqadmin.AuthoritySpec{
		Profile:     "APP.ORDERS",
		ObjectType:  mqadmin.AuthorityObjectTypeQueue,
		Principal:   "app",
		Authorities: []string{"GET", "PUT"},
	}
	if err := c.SetAuthority(context.Background(), spec); err != nil {
		t.Fatalf("SetAuthority: %v", err)
	}
	if !strings.Contains(lastCmd, "AUTHADD(GET,PUT)") {
		t.Fatalf("command = %q", lastCmd)
	}
	if err := c.DeleteAuthority(context.Background(), spec); err != nil {
		t.Fatalf("DeleteAuthority: %v", err)
	}
	if !strings.Contains(lastCmd, "AUTHRMV(ALL)") {
		t.Fatalf("command = %q", lastCmd)
	}
}

func TestClient_DeleteChannelAuthNotFound(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			testKeyOverallCompletionCode: 1,
			testKeyCommandResponse: []map[string]any{{
				testKeyCompletionCode: 1,
				"message":             []string{"AMQ8147E: Channel authority record not found"},
			}},
		})
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, srv.Client())
	spec := mqadmin.ChannelAuthSpec{
		ChannelName: "MISSING.CH",
		RuleType:    mqadmin.ChannelAuthRuleTypeAddressMap,
		Address:     "*",
	}
	if err := c.DeleteChannelAuth(context.Background(), spec); err != nil {
		t.Fatalf("DeleteChannelAuth: %v", err)
	}
}

func TestClient_SetChannelAuthInvalidSpec(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("RunMQSC should not be called for invalid spec")
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, srv.Client())
	err := c.SetChannelAuth(context.Background(), mqadmin.ChannelAuthSpec{})
	if err == nil {
		t.Fatal("expected invalid spec error")
	}
}

func TestClient_DeleteChannelAuthRealError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			testKeyOverallCompletionCode: 1,
			testKeyCommandResponse: []map[string]any{{
				testKeyCompletionCode: 1,
				"message":             []string{"AMQ1234E: permission denied"},
			}},
		})
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, srv.Client())
	spec := mqadmin.ChannelAuthSpec{
		ChannelName: "DEV.APP",
		RuleType:    mqadmin.ChannelAuthRuleTypeAddressMap,
		Address:     "*",
	}
	if err := c.DeleteChannelAuth(context.Background(), spec); err == nil {
		t.Fatal("expected error")
	}
}

func TestClient_SetAuthorityInvalidSpec(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("RunMQSC should not be called for invalid spec")
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, srv.Client())
	err := c.SetAuthority(context.Background(), mqadmin.AuthoritySpec{})
	if err == nil {
		t.Fatal("expected invalid spec error")
	}
}

func TestClient_DeleteAuthorityNotFound(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			testKeyOverallCompletionCode: 1,
			testKeyCommandResponse: []map[string]any{{
				testKeyCompletionCode: 1,
				"message":             []string{"AMQ8958E: authority record not found"},
			}},
		})
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, srv.Client())
	spec := mqadmin.AuthoritySpec{
		Profile:    "MISSING",
		ObjectType: mqadmin.AuthorityObjectTypeQueue,
		Principal:  "app",
	}
	if err := c.DeleteAuthority(context.Background(), spec); err != nil {
		t.Fatalf("DeleteAuthority: %v", err)
	}
}

func TestClient_GetChannelAuth(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			return
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		params, _ := body["parameters"].(map[string]any)
		cmd, _ := params["command"].(string)
		if !strings.Contains(cmd, "DISPLAY CHLAUTH") {
			t.Fatalf("command = %q", cmd)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			testKeyCommandResponse: []map[string]any{{
				testKeyCompletionCode: 0,
				"parameters": map[string]any{
					"address":  "*",
					"usersrc":  "CHANNEL",
					"chckclnt": "REQUIRED",
					"descr":    "test rule",
				},
			}},
			testKeyOverallCompletionCode: 0,
		})
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, srv.Client())
	spec := mqadmin.ChannelAuthSpec{
		ChannelName: "DEV.APP",
		RuleType:    mqadmin.ChannelAuthRuleTypeAddressMap,
	}
	state, err := c.GetChannelAuth(context.Background(), spec)
	if err != nil {
		t.Fatalf("GetChannelAuth: %v", err)
	}
	if state.Address != "*" || state.UserSource != "CHANNEL" {
		t.Fatalf("state = %+v", state)
	}
}

func TestClient_GetChannelAuthNotFound(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			testKeyCommandResponse: []map[string]any{{
				testKeyCompletionCode: 2,
				"message":             []string{"AMQ8147E: not found"},
			}},
			testKeyOverallCompletionCode: 2,
		})
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, srv.Client())
	spec := mqadmin.ChannelAuthSpec{
		ChannelName: "MISSING.CH",
		RuleType:    mqadmin.ChannelAuthRuleTypeAddressMap,
	}
	_, err := c.GetChannelAuth(context.Background(), spec)
	if err == nil || !errors.Is(err, mqadmin.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestClient_GetAuthority(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			return
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		params, _ := body["parameters"].(map[string]any)
		cmd, _ := params["command"].(string)
		if !strings.Contains(cmd, "DISPLAY AUTHREC") {
			t.Fatalf("command = %q", cmd)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			testKeyCommandResponse: []map[string]any{{
				testKeyCompletionCode: 0,
				"parameters":          map[string]any{"authlist": "GET,PUT"},
			}},
			testKeyOverallCompletionCode: 0,
		})
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, srv.Client())
	spec := mqadmin.AuthoritySpec{
		Profile:    "APP.ORDERS",
		ObjectType: mqadmin.AuthorityObjectTypeQueue,
		Principal:  "app",
	}
	state, err := c.GetAuthority(context.Background(), spec)
	if err != nil {
		t.Fatalf("GetAuthority: %v", err)
	}
	if len(state.Authorities) != 2 || state.Authorities[0] != "GET" {
		t.Fatalf("authorities = %v", state.Authorities)
	}
}

func TestClient_GetAuthorityNotFound(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			testKeyCommandResponse: []map[string]any{{
				testKeyCompletionCode: 2,
				"message":             []string{"AMQ8958E: not found"},
			}},
			testKeyOverallCompletionCode: 2,
		})
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, srv.Client())
	spec := mqadmin.AuthoritySpec{
		Profile:    "MISSING",
		ObjectType: mqadmin.AuthorityObjectTypeQueue,
		Principal:  "app",
	}
	_, err := c.GetAuthority(context.Background(), spec)
	if err == nil || !errors.Is(err, mqadmin.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestClient_GetChannelAuthInvalidSpec(t *testing.T) {
	t.Parallel()
	c := newTestClient(t, "https://example.invalid", http.DefaultClient)
	_, err := c.GetChannelAuth(context.Background(), mqadmin.ChannelAuthSpec{})
	if err == nil || !errors.Is(err, mqadmin.ErrTerminal) {
		t.Fatalf("expected terminal error, got %v", err)
	}
}

func TestClient_GetAuthorityInvalidSpec(t *testing.T) {
	t.Parallel()
	c := newTestClient(t, "https://example.invalid", http.DefaultClient)
	_, err := c.GetAuthority(context.Background(), mqadmin.AuthoritySpec{
		Profile:    "P",
		ObjectType: mqadmin.AuthorityObjectTypeQueue,
		Principal:  "a",
		Group:      "b",
	})
	if err == nil || !errors.Is(err, mqadmin.ErrTerminal) {
		t.Fatalf("expected terminal error, got %v", err)
	}
}

func TestClient_GetQueueUnsupportedType(t *testing.T) {
	t.Parallel()
	c := newTestClient(t, "https://example.invalid", http.DefaultClient)
	_, err := c.GetQueue(context.Background(), mqadmin.QueueSpec{
		Name: "APP.X",
		Type: mqadmin.QueueType("model"),
	})
	if err == nil || !errors.Is(err, mqadmin.ErrTerminal) {
		t.Fatalf("expected terminal error, got %v", err)
	}
}

func TestClient_GetAuthorityDisplayTextFallback(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			testKeyCommandResponse: []map[string]any{{
				testKeyCompletionCode: 0,
				"text":                []string{"AUTHLIST(GET,PUT)"},
			}},
			testKeyOverallCompletionCode: 0,
		})
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, srv.Client())
	spec := mqadmin.AuthoritySpec{
		Profile:    "APP.ORDERS",
		ObjectType: mqadmin.AuthorityObjectTypeQueue,
		Principal:  "app",
	}
	state, err := c.GetAuthority(context.Background(), spec)
	if err != nil {
		t.Fatalf("GetAuthority: %v", err)
	}
	if len(state.Authorities) == 0 {
		t.Fatalf("expected authorities from text display, state = %+v", state)
	}
}

func TestClient_DeleteChannelAuthInvalidSpec(t *testing.T) {
	t.Parallel()
	c := newTestClient(t, "https://example.invalid", http.DefaultClient)
	err := c.DeleteChannelAuth(context.Background(), mqadmin.ChannelAuthSpec{})
	if err == nil || !errors.Is(err, mqadmin.ErrTerminal) {
		t.Fatalf("expected terminal error, got %v", err)
	}
}

func TestClient_GetChannelAuthDisplayTextOnly(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			testKeyCommandResponse: []map[string]any{{
				testKeyCompletionCode: 0,
				"text":                []string{"ADDRESS(*)", "USERSRC(CHANNEL)"},
			}},
			testKeyOverallCompletionCode: 0,
		})
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, srv.Client())
	spec := mqadmin.ChannelAuthSpec{
		ChannelName: "DEV.APP",
		RuleType:    mqadmin.ChannelAuthRuleTypeAddressMap,
	}
	state, err := c.GetChannelAuth(context.Background(), spec)
	if err != nil {
		t.Fatalf("GetChannelAuth: %v", err)
	}
	if state.Address != "*" {
		t.Fatalf("state = %+v", state)
	}
}

func TestClient_GetAuthorityNoneAuthListNotFound(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			testKeyCommandResponse: []map[string]any{{
				testKeyCompletionCode: 0,
				"parameters": map[string]any{
					"authlist": "NONE",
				},
			}},
			testKeyOverallCompletionCode: 0,
		})
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, srv.Client())
	spec := mqadmin.AuthoritySpec{
		Profile:    "APP.NONE",
		ObjectType: mqadmin.AuthorityObjectTypeQueue,
		Principal:  "app",
	}
	_, err := c.GetAuthority(context.Background(), spec)
	if err == nil || !errors.Is(err, mqadmin.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestClient_GetAuthorityEmptyAuthListNotFound(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			testKeyCommandResponse: []map[string]any{{
				testKeyCompletionCode: 0,
				"parameters":          map[string]any{},
			}},
			testKeyOverallCompletionCode: 0,
		})
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, srv.Client())
	spec := mqadmin.AuthoritySpec{
		Profile:    "APP.EMPTY",
		ObjectType: mqadmin.AuthorityObjectTypeQueue,
		Principal:  "app",
	}
	_, err := c.GetAuthority(context.Background(), spec)
	if err == nil || !errors.Is(err, mqadmin.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
