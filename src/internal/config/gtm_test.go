package config

import (
	"testing"
	"time"
)

func TestLoadGTMConfig(t *testing.T) {
	cfg, err := LoadGTMConfig("../../../example/gitops-config/gtm/svc-plus.yaml")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Service != "svc-plus" {
		t.Fatalf("unexpected service: %s", cfg.Service)
	}

	if cfg.Domain != "api.svc.plus" {
		t.Fatalf("unexpected domain: %s", cfg.Domain)
	}

	if cfg.DNS.Provider != "cloudflare" || cfg.DNS.TTL != 30 {
		t.Fatalf("unexpected DNS settings: %+v", cfg.DNS)
	}

	if len(cfg.Regions) != 2 {
		t.Fatalf("expected 2 regions, got %d", len(cfg.Regions))
	}

	jp := cfg.Regions[0]
	if jp.Name != "jp" || jp.Weight != 100 || jp.MinReadyNodes != 1 || !jp.Fallback {
		t.Fatalf("unexpected jp region config: %+v", jp)
	}

	if cfg.Health.Type != "http" || cfg.Health.Path != "/healthz" {
		t.Fatalf("unexpected health config: %+v", cfg.Health)
	}

	if cfg.Health.Interval != 5*time.Second || cfg.Health.Timeout != 2*time.Second {
		t.Fatalf("unexpected health durations: %+v", cfg.Health)
	}
}

func TestLoadGTMConfigRejectsInvalid(t *testing.T) {
	if _, err := LoadGTMConfig("non-existent.yaml"); err == nil {
		t.Fatalf("expected error for missing file")
	}
}
