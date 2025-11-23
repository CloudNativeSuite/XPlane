package provider

type AliDNSProvider struct {
	*MemoryDNSProvider
}

func NewAliDNSProvider() *AliDNSProvider {
	return &AliDNSProvider{NewMemoryDNSProvider("alidns")}
}
