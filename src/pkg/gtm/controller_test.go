package gtm

import (
	"context"
	"testing"
	"time"

	"github.com/xplane/xplane/pkg/provider"
)

func TestReconcileComputesDynamicWeights(t *testing.T) {
	cf := provider.NewCloudflareProvider()
	ali := provider.NewAliDNSProvider()
	ctrl := NewGTMController(cf, ali)

	ctrl.RegisterRegion(Region{Name: "us-east", BaseWeight: 120, HealthThreshold: 0.05, MinReadyNodes: 2})

	err := ctrl.RegisterNode(Node{ID: "node-a", Region: "us-east", Address: "1.1.1.1", RTT: 50 * time.Millisecond, ErrorRate: 0.01, Status: NodeStatusUp})
	if err != nil {
		t.Fatalf("register node-a: %v", err)
	}
	_ = ctrl.RegisterNode(Node{ID: "node-b", Region: "us-east", Address: "1.1.1.2", RTT: 70 * time.Millisecond, ErrorRate: 0.02, Status: NodeStatusUp})
	_ = ctrl.RegisterNode(Node{ID: "node-c", Region: "us-east", Address: "1.1.1.3", RTT: 0, ErrorRate: 0.10, Status: NodeStatusDown})

	weights, err := ctrl.Reconcile(context.Background(), "api.example.com")
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	weight := weights["us-east"]
	if weight <= 0 {
		t.Fatalf("expected positive weight, got %f", weight)
	}

	snapshot := cf.Records()["api.example.com"]
	if snapshot["us-east"] != weight {
		t.Fatalf("cloudflare stored weight %f, expected %f", snapshot["us-east"], weight)
	}

	aliSnapshot := ali.Records()["api.example.com"]
	if aliSnapshot["us-east"] != weight {
		t.Fatalf("alidns stored weight %f, expected %f", aliSnapshot["us-east"], weight)
	}
}

func TestHeartbeatUpdatesNodeHealth(t *testing.T) {
	ctrl := NewGTMController()
	ctrl.RegisterRegion(Region{Name: "jp", BaseWeight: 80, HealthThreshold: 0.1, MinReadyNodes: 1})
	if err := ctrl.RegisterNode(Node{ID: "n1", Region: "jp", Status: NodeStatusUp}); err != nil {
		t.Fatalf("register node: %v", err)
	}

	if err := ctrl.Heartbeat("jp", "n1", 120*time.Millisecond, 0.2, true, NodeStatusDrain); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}

	snapshot := ctrl.Snapshot()["jp"]
	if snapshot.HealthyNodes != 0 {
		t.Fatalf("expected node to be unhealthy after heartbeat, got %d", snapshot.HealthyNodes)
	}
	if snapshot.LatencyFactor >= 0.8 {
		t.Fatalf("latency factor should degrade for slower RTT, got %f", snapshot.LatencyFactor)
	}
}
