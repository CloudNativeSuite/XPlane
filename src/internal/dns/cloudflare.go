package dns

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

type CloudflareProvider struct {
	APIToken string
	Client   *http.Client
}

func NewCloudflareProvider() *CloudflareProvider {
	return &CloudflareProvider{
		APIToken: os.Getenv("CF_API_TOKEN"),
		Client:   &http.Client{},
	}
}

func (p *CloudflareProvider) UpdateWeights(serviceConfig map[string]any, records []Record) error {
	accountID, _ := serviceConfig["account_id"].(string)
	lbID, _ := serviceConfig["load_balancer_id"].(string)

	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/load_balancers/%s", accountID, lbID)

	payload := map[string]any{
		"records": records,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+p.APIToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("cloudflare returned status %s", resp.Status)
	}
	return nil
}
