package config

import "testing"

func TestLoadGTMConfig(t *testing.T) {
	cfg, err := LoadGTMConfig("../../../example/gitops-config/gtm/gtm.yaml")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Domain != "api.xplane.local" {
		t.Fatalf("unexpected domain: %s", cfg.Domain)
	}

	if len(cfg.Providers) != 4 {
		t.Fatalf("expected 4 providers, got %d", len(cfg.Providers))
	}

	if len(cfg.Regions) != 2 {
		t.Fatalf("expected 2 regions, got %d", len(cfg.Regions))
	}

	usEast := cfg.Regions[0]
	if usEast.BaseWeight != 120 || usEast.MinReadyNodes != 2 {
		t.Fatalf("unexpected us-east config: %+v", usEast)
	}

	if len(usEast.Nodes) != 2 {
		t.Fatalf("expected 2 nodes in us-east, got %d", len(usEast.Nodes))
	}

	node := usEast.Nodes[0]
	if node.RTTMillis != 45 || node.Status != "up" {
		t.Fatalf("unexpected node values: %+v", node)
	}
}

func TestLoadGTMConfigRejectsInvalid(t *testing.T) {
	if _, err := LoadGTMConfig("non-existent.yaml"); err == nil {
		t.Fatalf("expected error for missing file")
	}
}
