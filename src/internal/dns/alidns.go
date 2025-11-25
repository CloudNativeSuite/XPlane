package dns

import "fmt"

type AliDNSProvider struct{}

func NewAliDNSProvider() *AliDNSProvider {
	return &AliDNSProvider{}
}

func (p *AliDNSProvider) UpdateWeights(serviceConfig map[string]any, records []Record) error {
	// TODO: integrate AliDNS SDK. Placeholder for now.
	fmt.Println("alidns UpdateWeights called", records)
	return nil
}
