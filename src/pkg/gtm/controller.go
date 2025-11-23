package gtm

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/xplane/xplane/pkg/provider"
)

// GTMController holds regions, nodes, and DNS providers while computing
// effective weights based on health and latency.
type GTMController struct {
	mu        sync.Mutex
	regions   map[string]*RegionState
	providers []provider.DNSProvider
}

// NewGTMController constructs a GTM controller with the provided DNS adapters.
func NewGTMController(providers ...provider.DNSProvider) *GTMController {
	return &GTMController{
		regions:   make(map[string]*RegionState),
		providers: providers,
	}
}

// RegisterRegion seeds static intent for a region.
func (c *GTMController) RegisterRegion(region Region) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.regions[region.Name]; !ok {
		c.regions[region.Name] = &RegionState{
			Region: region,
			Nodes:  make(map[string]*Node),
		}
		return
	}

	// allow updates to thresholds/base weights on an existing region
	state := c.regions[region.Name]
	state.Region.BaseWeight = region.BaseWeight
	state.Region.HealthThreshold = region.HealthThreshold
	state.Region.MinReadyNodes = region.MinReadyNodes
}

// RegisterNode adds or updates a node within its region and stamps registration
// timestamps when needed.
func (c *GTMController) RegisterNode(node Node) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	region, ok := c.regions[node.Region]
	if !ok {
		return fmt.Errorf("region %s not registered", node.Region)
	}

	if node.RegisteredAt.IsZero() {
		node.RegisteredAt = time.Now()
	}
	if node.Status == "" {
		node.Status = NodeStatusUp
	}
	// ensure map exists even if region predated the controller
	if region.Nodes == nil {
		region.Nodes = make(map[string]*Node)
	}

	region.Nodes[node.ID] = &node
	return nil
}

// Heartbeat refreshes runtime health data for a node.
func (c *GTMController) Heartbeat(regionName, nodeID string, rtt time.Duration, errorRate float64, blackbox bool, status NodeStatus) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	region, ok := c.regions[regionName]
	if !ok {
		return fmt.Errorf("region %s not registered", regionName)
	}

	node, ok := region.Nodes[nodeID]
	if !ok {
		return fmt.Errorf("node %s not registered in region %s", nodeID, regionName)
	}

	node.LastHeartbeat = time.Now()
	node.RTT = rtt
	node.ErrorRate = errorRate
	node.Blackbox = blackbox
	node.Status = status
	return nil
}

// RegionMetrics captures the computed telemetry that drives effective weight.
type RegionMetrics struct {
	TotalNodes         int
	ReadyNodes         int
	HealthyNodes       int
	AverageRTT         time.Duration
	HealthRatio        float64
	ReadyNodesFactor   float64
	LatencyFactor      float64
	EffectiveWeight    float64
	CurrentReadyNodes  int
	CurrentActiveNodes int
}

// computeMetrics derives health and weight numbers for a region.
func (c *GTMController) computeMetrics(state *RegionState) RegionMetrics {
	metrics := RegionMetrics{}
	totalRTT := time.Duration(0)
	latencySamples := 0

	metrics.TotalNodes = len(state.Nodes)
	for _, node := range state.Nodes {
		if node.Status == NodeStatusUp {
			metrics.ReadyNodes++
			metrics.CurrentReadyNodes++
		}
		if node.Status != NodeStatusDown {
			metrics.CurrentActiveNodes++
			totalRTT += node.RTT
			latencySamples++
		}
		if node.IsHealthy(state.Region.HealthThreshold) {
			metrics.HealthyNodes++
		}
	}

	if latencySamples > 0 {
		metrics.AverageRTT = totalRTT / time.Duration(latencySamples)
	}

	if metrics.TotalNodes > 0 {
		metrics.HealthRatio = float64(metrics.HealthyNodes) / float64(metrics.TotalNodes)
	}

	if state.Region.MinReadyNodes == 0 {
		metrics.ReadyNodesFactor = 1
	} else {
		metrics.ReadyNodesFactor = math.Min(1, float64(metrics.ReadyNodes)/float64(state.Region.MinReadyNodes))
	}

	metrics.LatencyFactor = computeLatencyFactor(metrics.AverageRTT)
	metrics.EffectiveWeight = state.Region.BaseWeight * metrics.HealthRatio * metrics.ReadyNodesFactor * metrics.LatencyFactor
	return metrics
}

// computeLatencyFactor converts an average RTT to a 0-1 weight.
func computeLatencyFactor(avgRTT time.Duration) float64 {
	if avgRTT <= 0 {
		return 1
	}
	normalized := float64(avgRTT.Milliseconds()) / 300.0
	if normalized < 0 {
		normalized = 0
	}
	return 1 / (1 + normalized)
}

// Reconcile computes the latest weights and pushes them to all configured DNS
// providers. Returns a map of region name to effective weight.
func (c *GTMController) Reconcile(ctx context.Context, domain string) (map[string]float64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	weights := make(map[string]float64, len(c.regions))
	for name, state := range c.regions {
		metrics := c.computeMetrics(state)
		weights[name] = metrics.EffectiveWeight
	}

	var applyErr error
	for _, provider := range c.providers {
		if err := provider.ApplyTrafficPolicy(ctx, domain, weights); err != nil {
			applyErr = errors.Join(applyErr, fmt.Errorf("provider %s: %w", provider.Name(), err))
		}
	}

	return weights, applyErr
}

// Snapshot returns a copy of the current region state for debugging or tests.
func (c *GTMController) Snapshot() map[string]RegionMetrics {
	c.mu.Lock()
	defer c.mu.Unlock()

	snapshot := make(map[string]RegionMetrics, len(c.regions))
	for name, state := range c.regions {
		snapshot[name] = c.computeMetrics(state)
	}
	return snapshot
}
