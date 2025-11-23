package provider

// CloudflareProvider wraps the shared memory implementation with a vendor
// label so GTM reports clearly which provider received updates.
type CloudflareProvider struct {
	*MemoryDNSProvider
}

// NewCloudflareProvider constructs a Cloudflare-compatible provider stub.
func NewCloudflareProvider() *CloudflareProvider {
	return &CloudflareProvider{NewMemoryDNSProvider("cloudflare")}
}
