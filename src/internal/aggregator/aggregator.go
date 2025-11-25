package aggregator

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/xplane/xplane/internal/dns"
	"github.com/xplane/xplane/internal/model"
)

type Aggregator struct {
	DB                  *sql.DB
	Providers           map[string]dns.Provider
	BlackboxURL         string
	NodeExporterPort    int
	ProcessExporterPort int
	Client              *http.Client
}

func New(db *sql.DB, providers map[string]dns.Provider) *Aggregator {
	return &Aggregator{
		DB:                  db,
		Providers:           providers,
		BlackboxURL:         envStr("BLACKBOX_URL", "http://blackbox:9115/probe"),
		NodeExporterPort:    envInt("NODE_EXPORTER_PORT", 9100),
		ProcessExporterPort: envInt("PROCESS_EXPORTER_PORT", 9256),
		Client:              &http.Client{Timeout: 5 * time.Second},
	}
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func parsePromText(text string) map[string]float64 {
	res := make(map[string]float64)
	sc := bufio.NewScanner(strings.NewReader(text))
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		if v, err := strconv.ParseFloat(fields[1], 64); err == nil {
			res[fields[0]] = v
		}
	}
	return res
}

func (a *Aggregator) probeBlackbox(ip string, port *int) (success bool, latencyMs float64, raw string, err error) {
	target := fmt.Sprintf("http://%s", ip)
	if port != nil {
		target = fmt.Sprintf("http://%s:%d", ip, *port)
	}
	req, _ := http.NewRequest(http.MethodGet, a.BlackboxURL, nil)
	q := req.URL.Query()
	q.Set("target", target)
	q.Set("module", "http_2xx")
	req.URL.RawQuery = q.Encode()

	resp, err := a.Client.Do(req)
	if err != nil {
		return false, 9999, "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	raw = string(data)
	m := parsePromText(raw)
	success = m["probe_success"] == 1
	latencySec := m["probe_duration_seconds"]
	latencyMs = latencySec * 1000
	return
}

func (a *Aggregator) probeNodeExporter(ip string) (load1 float64, raw string, err error) {
	url := fmt.Sprintf("http://%s:%d/metrics", ip, a.NodeExporterPort)
	resp, err := a.Client.Get(url)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	raw = string(data)
	m := parsePromText(raw)
	return m["node_load1"], raw, nil
}

func (a *Aggregator) probeProcessExporter(ip string) (running bool, raw string, err error) {
	url := fmt.Sprintf("http://%s:%d/metrics", ip, a.ProcessExporterPort)
	resp, err := a.Client.Get(url)
	if err != nil {
		return false, "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	raw = string(data)
	m := parsePromText(raw)
	_, ok := m["process_start_time_seconds"]
	return ok, raw, nil
}

type evalResult struct {
	Status          string
	EffectiveWeight int
}

func evaluate(baseWeight int, bbOK bool, latencyMs float64, load1 float64, running bool) evalResult {
	if !bbOK || !running {
		return evalResult{"down", 0}
	}
	if load1 > 8.0 {
		return evalResult{"drain", 20}
	}
	w := 1000.0 / (latencyMs + 50.0)
	if w < 10 {
		w = 10
	}
	total := int(w * float64(baseWeight) / 100.0)
	if total < 0 {
		total = 0
	}
	return evalResult{"up", total}
}

func (a *Aggregator) RunOnce(ctx context.Context) error {
	rows, err := a.DB.QueryContext(ctx, "SELECT id, name, domain, dns_provider, dns_config, created_at FROM services")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var svc model.Service
		if err := rows.Scan(&svc.ID, &svc.Name, &svc.Domain, &svc.DNSProvider, &svc.DNSConfig, &svc.CreatedAt); err != nil {
			return err
		}
		provider, ok := a.Providers[svc.DNSProvider]
		if !ok {
			continue
		}

		nrows, err := a.DB.QueryContext(ctx, `
SELECT id, service_id, ip, port, region, role, base_weight, status, latency_ms, cpu_load, last_seen_at, created_at
FROM nodes WHERE service_id=$1
`, svc.ID)
		if err != nil {
			return err
		}

		var records []dns.Record

		now := time.Now().UTC()
		for nrows.Next() {
			var node model.Node
			var port sql.NullInt64
			var region sql.NullString
			var latency sql.NullFloat64
			var cpu sql.NullFloat64
			var lastSeen sql.NullTime

			if err := nrows.Scan(&node.ID, &node.ServiceID, &node.IP, &port, &region, &node.Role, &node.BaseWeight, &node.Status, &latency, &cpu, &lastSeen, &node.CreatedAt); err != nil {
				nrows.Close()
				return err
			}
			if port.Valid {
				p := int(port.Int64)
				node.Port = &p
			}
			if region.Valid {
				r := region.String
				node.Region = &r
			}

			bbOK, latencyMs, _, err := a.probeBlackbox(node.IP, node.Port)
			if err != nil {
				bbOK = false
				latencyMs = 9999
			}
			load1, _, err := a.probeNodeExporter(node.IP)
			if err != nil {
				load1 = 0
			}
			running, _, err := a.probeProcessExporter(node.IP)
			if err != nil {
				running = true
			}

			ev := evaluate(node.BaseWeight, bbOK, latencyMs, load1, running)

			if _, err = a.DB.ExecContext(ctx, `
UPDATE nodes
SET status=$1, latency_ms=$2, cpu_load=$3, last_seen_at=$4
WHERE id=$5
`, ev.Status, latencyMs, load1, now, node.ID); err != nil {
				nrows.Close()
				return err
			}

			records = append(records, dns.Record{
				IP:     node.IP,
				Weight: ev.EffectiveWeight,
				ID:     fmt.Sprintf("%d-%s", node.ID, valueOr(node.Region, "default")),
			})
		}
		nrows.Close()

		if len(records) > 0 {
			cfgMap := map[string]any{}
			if err := provider.UpdateWeights(cfgMap, records); err != nil {
				fmt.Println("update weights error:", err)
			}
		}
	}

	return rows.Err()
}

func valueOr(p *string, def string) string {
	if p != nil && *p != "" {
		return *p
	}
	return def
}
