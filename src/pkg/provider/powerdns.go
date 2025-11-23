package provider

type PowerDNSProvider struct {
	*MemoryDNSProvider
}

func NewPowerDNSProvider() *PowerDNSProvider {
	return &PowerDNSProvider{NewMemoryDNSProvider("powerdns")}
}
