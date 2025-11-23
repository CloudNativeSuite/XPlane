package gtm

import "time"

// NodeStatus describes runtime availability for routing decisions.
type NodeStatus string

const (
	NodeStatusUp    NodeStatus = "up"
	NodeStatusDown  NodeStatus = "down"
	NodeStatusDrain NodeStatus = "drain"
)

// Node represents a single edge endpoint being tracked by the GTM controller.
type Node struct {
	ID            string
	Region        string
	Address       string
	RegisteredAt  time.Time
	LastHeartbeat time.Time
	RTT           time.Duration
	ErrorRate     float64
	Blackbox      bool
	Status        NodeStatus
}

// IsHealthy evaluates error rate and status against a region's health
// threshold. Blackboxed nodes are treated as unhealthy until monitoring is
// restored.
func (n Node) IsHealthy(healthThreshold float64) bool {
	if n.Status != NodeStatusUp {
		return false
	}
	if n.Blackbox {
		return false
	}
	return n.ErrorRate <= healthThreshold
}

// Region captures static GTM intent and dynamic node membership.
type Region struct {
	Name            string
	BaseWeight      float64
	HealthThreshold float64
	MinReadyNodes   int
}

// RegionState keeps runtime telemetry about a region and its nodes.
type RegionState struct {
	Region Region
	Nodes  map[string]*Node
}
