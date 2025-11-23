package provider

import (
	"context"
	"sync"
)

// DNSProvider applies weighted DNS updates for GTM decisions.
type DNSProvider interface {
	Name() string
	// ApplyTrafficPolicy updates a domain's regional weights. Implementations
	// should treat the weights map as authoritative for the domain.
	ApplyTrafficPolicy(ctx context.Context, domain string, weights map[string]float64) error
}

// MemoryDNSProvider is a lightweight in-memory implementation shared by
// vendor-specific wrappers so the controller can be exercised without
// external credentials.
type MemoryDNSProvider struct {
	vendor  string
	mu      sync.Mutex
	records map[string]map[string]float64
}

// NewMemoryDNSProvider constructs a new memory-backed provider.
func NewMemoryDNSProvider(vendor string) *MemoryDNSProvider {
	return &MemoryDNSProvider{
		vendor:  vendor,
		records: make(map[string]map[string]float64),
	}
}

// Name returns the provider vendor label.
func (p *MemoryDNSProvider) Name() string {
	return p.vendor
}

// ApplyTrafficPolicy records the desired weights for the domain.
func (p *MemoryDNSProvider) ApplyTrafficPolicy(_ context.Context, domain string, weights map[string]float64) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	copy := make(map[string]float64, len(weights))
	for region, weight := range weights {
		copy[region] = weight
	}
	p.records[domain] = copy
	return nil
}

// Records returns a snapshot of the last applied weights for inspection in
// tests or diagnostics.
func (p *MemoryDNSProvider) Records() map[string]map[string]float64 {
	p.mu.Lock()
	defer p.mu.Unlock()

	snapshot := make(map[string]map[string]float64, len(p.records))
	for domain, weights := range p.records {
		weightCopy := make(map[string]float64, len(weights))
		for region, weight := range weights {
			weightCopy[region] = weight
		}
		snapshot[domain] = weightCopy
	}
	return snapshot
}
