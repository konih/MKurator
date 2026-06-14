package logging_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/conduit-ops/mkurator/internal/logging"
)

func TestLogLevelFiltersVerbose(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		level     logging.Level
		wantDebug bool
	}{
		{name: "info hides V1", level: logging.LevelInfo, wantDebug: false},
		{name: "debug shows V1", level: logging.LevelDebug, wantDebug: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			logger, err := logging.NewLogger(logging.Config{
				Level:  tt.level,
				Format: logging.FormatJSON,
			}, &buf)
			if err != nil {
				t.Fatalf("NewLogger: %v", err)
			}

			logger.Info("always visible")
			logger.V(1).Info("verbose detail")

			lines := nonEmptyLines(buf.String())
			if len(lines) != 1 && !tt.wantDebug {
				t.Fatalf("got %d log lines, want 1:\n%s", len(lines), buf.String())
			}
			if tt.wantDebug && len(lines) != 2 {
				t.Fatalf("got %d log lines, want 2:\n%s", len(lines), buf.String())
			}

			var entry map[string]any
			if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
				t.Fatalf("unmarshal info line: %v", err)
			}
			if entry["msg"] != "always visible" {
				t.Fatalf("msg: got %v", entry["msg"])
			}
			if tt.wantDebug {
				if err := json.Unmarshal([]byte(lines[1]), &entry); err != nil {
					t.Fatalf("unmarshal debug line: %v", err)
				}
				if entry["msg"] != "verbose detail" {
					t.Fatalf("verbose msg: got %v", entry["msg"])
				}
				level, _ := entry["level"].(string)
				if !strings.Contains(level, "DEBUG") {
					t.Fatalf("verbose level: got %v want DEBUG (logr V(1))", entry["level"])
				}
			}
		})
	}
}

func TestSampleJSONLogOutput(t *testing.T) {
	var buf bytes.Buffer
	logger, err := logging.NewLogger(logging.Config{
		Level:  logging.LevelInfo,
		Format: logging.FormatJSON,
	}, &buf)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}

	setup := logger.WithName("setup")
	setup.Info("starting manager", "controller", "manager")

	line := strings.TrimSpace(buf.String())
	var entry map[string]any
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		t.Fatalf("sample JSON not valid: %v\nraw: %s", err, line)
	}

	for _, key := range []string{"time", "level", "msg", "logger", "controller"} {
		if _, ok := entry[key]; !ok {
			t.Fatalf("missing key %q in sample output: %v", key, entry)
		}
	}
	if entry["level"] != "INFO" {
		t.Fatalf("level: got %v", entry["level"])
	}
	if entry["msg"] != "starting manager" {
		t.Fatalf("msg: got %v", entry["msg"])
	}
	if entry["logger"] != "setup" {
		t.Fatalf("logger: got %v", entry["logger"])
	}
	if entry["controller"] != "manager" {
		t.Fatalf("controller: got %v", entry["controller"])
	}

	t.Logf("sample JSON log line:\n%s", line)
}

func TestSampleTextLogOutput(t *testing.T) {
	var buf bytes.Buffer
	logger, err := logging.NewLogger(logging.Config{
		Level:  logging.LevelInfo,
		Format: logging.FormatText,
	}, &buf)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}

	logger.Info("starting manager", "controller", "manager")

	out := buf.String()
	for _, want := range []string{"level=INFO", `msg="starting manager"`, "controller=manager"} {
		if !strings.Contains(out, want) {
			t.Fatalf("text output missing %q:\n%s", want, out)
		}
	}
	t.Logf("sample text log line:\n%s", strings.TrimSpace(out))
}

func TestSetupWithWriter_InvalidLevel(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	err := logging.SetupWithWriter(logging.Config{
		Level:  logging.Level("bogus"),
		Format: logging.FormatJSON,
	}, &buf)
	if err == nil {
		t.Fatal("expected error for invalid log level")
	}
}

func nonEmptyLines(s string) []string {
	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
