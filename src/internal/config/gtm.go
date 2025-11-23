package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// GTMConfig captures the desired DNS traffic policy and region composition.
type GTMConfig struct {
	Domain    string         `yaml:"domain"`
	Providers []string       `yaml:"providers"`
	Regions   []RegionConfig `yaml:"regions"`
}

// RegionConfig mirrors the GTM controller's region intent.
type RegionConfig struct {
	Name            string       `yaml:"name"`
	BaseWeight      float64      `yaml:"base_weight"`
	HealthThreshold float64      `yaml:"health_threshold"`
	MinReadyNodes   int          `yaml:"min_ready_nodes"`
	Nodes           []NodeConfig `yaml:"nodes"`
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

	if cfg.Domain == "" {
		return GTMConfig{}, fmt.Errorf("config missing domain")
	}
	if len(cfg.Providers) == 0 {
		return GTMConfig{}, fmt.Errorf("config must declare at least one provider")
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
	if region.BaseWeight <= 0 {
		return fmt.Errorf("base_weight must be greater than zero")
	}
	if region.HealthThreshold < 0 {
		return fmt.Errorf("health_threshold cannot be negative")
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

// RTT converts the configured RTT to a duration for the controller.
func (n NodeConfig) RTT() time.Duration {
	return time.Duration(n.RTTMillis) * time.Millisecond
}
