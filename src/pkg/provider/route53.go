package provider

type Route53Provider struct {
	*MemoryDNSProvider
}

func NewRoute53Provider() *Route53Provider {
	return &Route53Provider{NewMemoryDNSProvider("route53")}
}
