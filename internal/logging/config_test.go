package logging_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/conduit-ops/mkurator/internal/logging"
)

func TestLoadDefaultsLocal(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	t.Setenv(logging.EnvConfig, "")
	t.Setenv(logging.EnvLevel, "")
	t.Setenv(logging.EnvFormat, "")

	cfg, err := logging.Load(logging.Options{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Level != logging.LevelInfo {
		t.Fatalf("level: got %q want info", cfg.Level)
	}
	if cfg.Format != logging.FormatText {
		t.Fatalf("format: got %q want text", cfg.Format)
	}
}

func TestLoadDefaultsInCluster(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "10.0.0.1")
	t.Setenv(logging.EnvConfig, "")
	t.Setenv(logging.EnvLevel, "")
	t.Setenv(logging.EnvFormat, "")

	cfg, err := logging.Load(logging.Options{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Format != logging.FormatJSON {
		t.Fatalf("format: got %q want json", cfg.Format)
	}
}

func TestLoadPrecedence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "logging.yaml")
	if err := os.WriteFile(path, []byte("level: warn\nformat: text\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	t.Setenv(logging.EnvConfig, path)
	t.Setenv(logging.EnvLevel, "error")
	t.Setenv(logging.EnvFormat, "json")

	cfg, err := logging.Load(logging.Options{Level: "debug"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Level != logging.LevelDebug {
		t.Fatalf("level: got %q want debug (flag wins)", cfg.Level)
	}
	if cfg.Format != logging.FormatJSON {
		t.Fatalf("format: got %q want json (env wins over file)", cfg.Format)
	}
}

func TestLoadInvalidLevel(t *testing.T) {
	_, err := logging.Load(logging.Options{Level: "trace"})
	if err == nil {
		t.Fatal("expected error for invalid level")
	}
}

func TestLoadInvalidFormat(t *testing.T) {
	_, err := logging.Load(logging.Options{Format: "xml"})
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
}

func TestLoadInvalidConfigFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "logging.yaml")
	if err := os.WriteFile(path, []byte("level: [\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(logging.EnvConfig, path)
	_, err := logging.Load(logging.Options{})
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoadFileNotFound(t *testing.T) {
	t.Setenv(logging.EnvConfig, "/nonexistent/logging.yaml")
	_, err := logging.Load(logging.Options{})
	if err == nil {
		t.Fatal("expected error for missing config file")
	}
}

func TestLoadFileInvalidLevelInFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "logging.yaml")
	if err := os.WriteFile(path, []byte("level: trace\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(logging.EnvConfig, path)
	_, err := logging.Load(logging.Options{})
	if err == nil {
		t.Fatal("expected error for invalid level in file")
	}
}

func TestSetup(t *testing.T) {
	if err := logging.Setup(logging.Config{
		Level:  logging.LevelInfo,
		Format: logging.FormatJSON,
	}); err != nil {
		t.Fatalf("Setup JSON: %v", err)
	}
	if err := logging.Setup(logging.Config{
		Level:  logging.LevelInfo,
		Format: logging.FormatText,
	}); err != nil {
		t.Fatalf("Setup text: %v", err)
	}
}

func TestSetupWithWriter(t *testing.T) {
	var buf bytes.Buffer
	if err := logging.SetupWithWriter(logging.Config{
		Level:  logging.LevelWarn,
		Format: logging.FormatText,
	}, &buf); err != nil {
		t.Fatalf("SetupWithWriter: %v", err)
	}
	logger, err := logging.NewLogger(logging.Config{
		Level:  logging.LevelError,
		Format: logging.FormatJSON,
	}, &buf)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	logger.Info("ignored at error level")
	if buf.Len() != 0 {
		t.Fatalf("expected no output at error level for info, got %q", buf.String())
	}
}

func TestNewLoggerUnsupportedLevel(t *testing.T) {
	_, err := logging.NewLogger(logging.Config{
		Level:  logging.Level("trace"),
		Format: logging.FormatJSON,
	}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error for unsupported level")
	}
}

func TestNewHandlerUnsupportedFormat(t *testing.T) {
	_, err := logging.NewHandler(logging.Config{
		Level:  logging.LevelInfo,
		Format: logging.Format("xml"),
	}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error for unsupported format")
	}
}
