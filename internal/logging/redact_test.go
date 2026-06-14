package logging_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/conduit-ops/mkurator/internal/logging"
)

func TestRedactHandlerWithSlogLogger(t *testing.T) {
	var buf bytes.Buffer
	handler, err := logging.NewHandler(logging.Config{
		Level:  logging.LevelInfo,
		Format: logging.FormatJSON,
	}, &buf)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	slog.New(handler).Info("event", slog.String("token", "abc123"))

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if entry["token"] != "[REDACTED]" {
		t.Fatalf("token: got %v", entry["token"])
	}
}

func TestRedactHandlerSensitiveKeys(t *testing.T) {
	t.Parallel()
	keys := []string{
		"password", "passwd", "token", "authorization", "secret",
		"credentials", "csrf", "apikey", "api_key",
	}
	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			handler, err := logging.NewHandler(logging.Config{
				Level:  logging.LevelInfo,
				Format: logging.FormatJSON,
			}, &buf)
			if err != nil {
				t.Fatalf("NewHandler: %v", err)
			}
			slog.New(handler).Info("event", slog.String(key, "leak"))
			var entry map[string]any
			if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if entry[key] != "[REDACTED]" {
				t.Fatalf("%s: got %v", key, entry[key])
			}
		})
	}
}

func TestRedactHandlerWithAttrsAndGroup(t *testing.T) {
	var buf bytes.Buffer
	handler, err := logging.NewHandler(logging.Config{
		Level:  logging.LevelInfo,
		Format: logging.FormatJSON,
	}, &buf)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	child := handler.WithAttrs([]slog.Attr{slog.String("password", "secret")}).WithGroup("conn")
	slog.New(child).Info("connected", slog.String("channel", "DEV.APP.SVRCONN"))

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if entry["password"] != "[REDACTED]" {
		t.Fatalf("password: got %v", entry["password"])
	}
	conn, ok := entry["conn"].(map[string]any)
	if !ok {
		t.Fatalf("conn group: %v", entry)
	}
	if conn["channel"] != "DEV.APP.SVRCONN" {
		t.Fatalf("channel: got %v", conn["channel"])
	}
}
