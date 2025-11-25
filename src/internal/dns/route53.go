package dns

import (
	"context"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	r53 "github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
)

type Route53Provider struct {
	Client *r53.Client
}

func NewRoute53Provider() (*Route53Provider, error) {
	region := os.Getenv("AWS_REGION")
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region))
	if err != nil {
		return nil, err
	}
	return &Route53Provider{Client: r53.NewFromConfig(cfg)}, nil
}

func (p *Route53Provider) UpdateWeights(serviceConfig map[string]any, records []Record) error {
	hostedZoneID, _ := serviceConfig["hosted_zone_id"].(string)
	domain, _ := serviceConfig["domain"].(string)
	ttl := int64(30)
	if t, ok := serviceConfig["ttl"].(float64); ok {
		ttl = int64(t)
	}

	changes := make([]r53types.Change, 0, len(records))
	for _, r := range records {
		changes = append(changes, r53types.Change{
			Action: r53types.ChangeActionUpsert,
			ResourceRecordSet: &r53types.ResourceRecordSet{
				Name:          aws.String(domain),
				Type:          r53types.RRTypeA,
				SetIdentifier: aws.String(r.ID),
				Weight:        aws.Int64(int64(r.Weight)),
				TTL:           aws.Int64(ttl),
				ResourceRecords: []r53types.ResourceRecord{
					{Value: aws.String(r.IP)},
				},
			},
		})
	}

	_, err := p.Client.ChangeResourceRecordSets(context.Background(), &r53.ChangeResourceRecordSetsInput{
		HostedZoneId: aws.String(hostedZoneID),
		ChangeBatch:  &r53types.ChangeBatch{Changes: changes},
	})
	return err
}
