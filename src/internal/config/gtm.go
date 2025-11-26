package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// GTMConfig captures the desired DNS traffic policy and region composition.
type GTMConfig struct {
	Service string         `yaml:"service"`
	Domain  string         `yaml:"domain"`
	Regions []RegionConfig `yaml:"regions"`
	Health  HealthConfig   `yaml:"health"`
	DNS     DNSConfig      `yaml:"dns"`
}

// RegionConfig mirrors the GTM controller's region intent.
type RegionConfig struct {
	Name          string       `yaml:"name"`
	Weight        float64      `yaml:"weight"`
	MinReadyNodes int          `yaml:"min_ready_nodes"`
	Fallback      bool         `yaml:"fallback"`
	Nodes         []NodeConfig `yaml:"nodes,omitempty"`
}

// NodeConfig represents the desired membership for a region.
type NodeConfig struct {
	ID        string  `yaml:"id"`
	Address   string  `yaml:"address"`
	RTTMillis int     `yaml:"rtt_ms"`
	ErrorRate float64 `yaml:"error_rate"`
	Blackbox  bool    `yaml:"blackbox"`
	Status    string  `yaml:"status"`
}

// HealthConfig declares the endpoint probing strategy for GTM health.
type HealthConfig struct {
	Type     string        `yaml:"type"`
	Path     string        `yaml:"path"`
	Interval time.Duration `yaml:"interval"`
	Timeout  time.Duration `yaml:"timeout"`
}

// DNSConfig captures provider-specific DNS settings.
type DNSConfig struct {
	Provider string `yaml:"provider"`
	TTL      int    `yaml:"ttl"`
}

// LoadGTMConfig reads and parses a YAML-formatted GTM configuration file.
func LoadGTMConfig(path string) (GTMConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return GTMConfig{}, fmt.Errorf("read config: %w", err)
	}

	var cfg GTMConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return GTMConfig{}, fmt.Errorf("parse config: %w", err)
	}

	if cfg.Service == "" {
		return GTMConfig{}, fmt.Errorf("config missing service")
	}
	if cfg.Domain == "" {
		return GTMConfig{}, fmt.Errorf("config missing domain")
	}
	if cfg.DNS.Provider == "" {
		return GTMConfig{}, fmt.Errorf("config missing DNS provider")
	}
	if cfg.DNS.TTL <= 0 {
		return GTMConfig{}, fmt.Errorf("config ttl must be greater than zero")
	}
	if err := validateHealth(cfg.Health); err != nil {
		return GTMConfig{}, fmt.Errorf("health: %w", err)
	}
	if len(cfg.Regions) == 0 {
		return GTMConfig{}, fmt.Errorf("config must declare at least one region")
	}

	for i := range cfg.Regions {
		if err := validateRegion(&cfg.Regions[i]); err != nil {
			return GTMConfig{}, fmt.Errorf("region %d: %w", i, err)
		}
	}

	return cfg, nil
}

func validateRegion(region *RegionConfig) error {
	if region.Name == "" {
		return fmt.Errorf("name is required")
	}
	if region.Weight <= 0 {
		return fmt.Errorf("weight must be greater than zero")
	}
	if region.MinReadyNodes < 0 {
		return fmt.Errorf("min_ready_nodes cannot be negative")
	}

	for i := range region.Nodes {
		n := &region.Nodes[i]
		if n.ID == "" {
			return fmt.Errorf("node %d: id is required", i)
		}
		if n.RTTMillis < 0 {
			return fmt.Errorf("node %d: rtt_ms cannot be negative", i)
		}
		// normalize empty status to avoid later parsing errors
		if n.Status == "" {
			n.Status = "up"
		}
	}

	return nil
}

func validateHealth(health HealthConfig) error {
	if health.Type == "" {
		return fmt.Errorf("type is required")
	}
	if health.Path == "" {
		return fmt.Errorf("path is required")
	}
	if health.Interval <= 0 {
		return fmt.Errorf("interval must be greater than zero")
	}
	if health.Timeout <= 0 {
		return fmt.Errorf("timeout must be greater than zero")
	}
	return nil
}

// RTT converts the configured RTT to a duration for the controller.
func (n NodeConfig) RTT() time.Duration {
	return time.Duration(n.RTTMillis) * time.Millisecond
}
