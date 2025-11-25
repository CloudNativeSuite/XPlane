package main

import (
	"context"
	"log"
	"time"

	"github.com/xplane/xplane/internal/aggregator"
	"github.com/xplane/xplane/internal/db"
	"github.com/xplane/xplane/internal/dns"
)

func main() {
	d, err := db.Open()
	if err != nil {
		log.Fatal(err)
	}
	if err := db.Migrate(d); err != nil {
		log.Fatal(err)
	}

	providers := map[string]dns.Provider{
		"cloudflare": dns.NewCloudflareProvider(),
	}

	agg := aggregator.New(d, providers)

	ctx := context.Background()
	interval := 10 * time.Second

	log.Println("aggregator started, interval", interval)
	for {
		if err := agg.RunOnce(ctx); err != nil {
			log.Println("aggregator error:", err)
		}
		time.Sleep(interval)
	}
}
