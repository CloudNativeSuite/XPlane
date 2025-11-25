package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/xplane/xplane/internal/config"
	"github.com/xplane/xplane/pkg/gtm"
	"github.com/xplane/xplane/pkg/provider"
)

func main() {
	cfgPath := flag.String("config", "../example/gitops-config/gtm/svc-plus.yaml", "Path to GTM configuration file")
	interval := flag.Duration("interval", 30*time.Second, "Reconcile interval")
	runOnce := flag.Bool("once", false, "Run a single reconcile and exit")
	flag.Parse()

	logger := log.New(os.Stdout, "gtm-controller ", log.LstdFlags|log.Lmsgprefix)

	cfg, err := config.LoadGTMConfig(*cfgPath)
	if err != nil {
		logger.Fatalf("load config: %v", err)
	}

	p := buildProvider(cfg.DNS.Provider)
	if p == nil {
		logger.Fatalf("unknown DNS provider %q", cfg.DNS.Provider)
	}

	controller := gtm.NewGTMController(p)
	if err := seedController(controller, cfg); err != nil {
		logger.Fatalf("seed controller: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	runReconcile(ctx, logger, controller, cfg.Domain)
	if *runOnce {
		return
	}

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Println("shutdown requested; exiting")
			return
		case <-ticker.C:
			runReconcile(ctx, logger, controller, cfg.Domain)
		}
	}
}

func buildProvider(name string) provider.DNSProvider {
	switch name {
	case "cloudflare":
		return provider.NewCloudflareProvider()
	case "alidns":
		return provider.NewAliDNSProvider()
	case "route53":
		return provider.NewRoute53Provider()
	case "powerdns":
		return provider.NewPowerDNSProvider()
	default:
		return nil
	}
}

func seedController(controller *gtm.GTMController, cfg config.GTMConfig) error {
	for _, region := range cfg.Regions {
		controller.RegisterRegion(gtm.Region{
			Name:            region.Name,
			BaseWeight:      region.Weight,
			HealthThreshold: 0,
			MinReadyNodes:   region.MinReadyNodes,
		})

		for _, node := range region.Nodes {
			status, err := gtm.NodeStatusFromString(node.Status)
			if err != nil {
				return err
			}
			if err := controller.RegisterNode(gtm.Node{
				ID:        node.ID,
				Region:    region.Name,
				Address:   node.Address,
				RTT:       node.RTT(),
				ErrorRate: node.ErrorRate,
				Blackbox:  node.Blackbox,
				Status:    status,
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

func runReconcile(ctx context.Context, logger *log.Logger, controller *gtm.GTMController, domain string) {
	weights, err := controller.Reconcile(ctx, domain)
	if err != nil {
		logger.Printf("reconcile error: %v", err)
	}

	for region, weight := range weights {
		logger.Printf("region=%s weight=%.2f", region, weight)
	}
}
