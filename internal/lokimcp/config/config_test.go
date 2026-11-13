package config

import (
	"os"
	"testing"
	"time"
)

func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"LOKI_BASE_URL", "PROM_BASE_URL", "PROM_MAX_POINTS", "AWS_READ_ENABLED", "AWS_READ_CLI_PATH", "AWS_READ_REGION", "AWS_READ_TIMEOUT"} {
		os.Unsetenv(k)
	}
}

func TestLoadMCP_RequiresAtLeastOneBackend(t *testing.T) {
	clearEnv(t)
	if _, err := LoadMCP(); err == nil {
		t.Fatal("expected error when neither LOKI_BASE_URL nor PROM_BASE_URL is set")
	}
}

func TestLoadMCP_PromOnlyIsValid(t *testing.T) {
	clearEnv(t)
	os.Setenv("PROM_BASE_URL", "http://localhost:9090")
	defer os.Unsetenv("PROM_BASE_URL")
	cfg, err := LoadMCP()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.PromBaseURL != "http://localhost:9090" {
		t.Fatalf("PromBaseURL = %q", cfg.PromBaseURL)
	}
	if cfg.PromMaxPoints != 11000 {
		t.Fatalf("PromMaxPoints default = %d, want 11000", cfg.PromMaxPoints)
	}
}

func TestLoadMCP_LokiOnlyStillValid(t *testing.T) {
	clearEnv(t)
	os.Setenv("LOKI_BASE_URL", "http://localhost:3100")
	defer os.Unsetenv("LOKI_BASE_URL")
	if _, err := LoadMCP(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadMCP_AWSReadDefaults(t *testing.T) {
	clearEnv(t)
	os.Setenv("LOKI_BASE_URL", "http://localhost:3100")
	defer os.Unsetenv("LOKI_BASE_URL")
	cfg, err := LoadMCP()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AWSReadEnabled || cfg.AWSReadCLIPath != "aws" || cfg.AWSReadRegion != "eu-west-1" || cfg.AWSReadTimeout != 30*time.Second {
		t.Fatalf("defaults = %+v", cfg)
	}
}

func TestLoadMCP_AWSReadOnlyIsValid(t *testing.T) {
	clearEnv(t)
	os.Setenv("AWS_READ_ENABLED", "true")
	os.Setenv("AWS_READ_REGION", "eu-central-1")
	os.Setenv("AWS_READ_TIMEOUT", "5s")
	defer clearEnv(t)
	cfg, err := LoadMCP()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.AWSReadEnabled || cfg.AWSReadRegion != "eu-central-1" || cfg.AWSReadTimeout != 5*time.Second {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestParseDatasourceMap(t *testing.T) {
	got, err := parseDatasourceMap(" grey-db-prod-replica:card-db=pg-abc , grey-db-audit-prod=pg-def,")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got["grey-db-prod-replica:card-db"] != "pg-abc" || got["grey-db-audit-prod"] != "pg-def" {
		t.Fatalf("got %v", got)
	}
	if empty, err := parseDatasourceMap(""); err != nil || len(empty) != 0 {
		t.Fatalf("empty: %v %v", empty, err)
	}
	if _, err := parseDatasourceMap("missing-uid"); err == nil {
		t.Fatal("expected error for entry without =")
	}
}
