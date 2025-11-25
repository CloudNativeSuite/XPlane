package dns

type Record struct {
	IP     string
	Weight int
	ID     string
}

type Provider interface {
	UpdateWeights(serviceConfig map[string]any, records []Record) error
}
