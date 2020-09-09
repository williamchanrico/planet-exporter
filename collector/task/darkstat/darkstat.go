package darkstat

import (
	"context"
	"fmt"
	"planet-exporter/pkg/prometheus"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/prom2json"
	log "github.com/sirupsen/logrus"
)

type task struct {
	darkstatAddr string

	mu     sync.Mutex
	values []Metric
}

var cache task

func InitTask(ctx context.Context, darkstatAddr string) {
	cache = task{
		darkstatAddr: darkstatAddr,
	}
}

// Metric contains values needed for prometheus metrics
type Metric struct {
	Protocol  string // tcp/udp
	Name      string // e.g. hostgroup
	Domain    string // e.g. consul domain
	Port      string
	Direction string // in or out
	Bandwidth float64
}

// Get returns latest metrics in cache.values
func Get() []Metric {
	cache.mu.Lock()
	darkstats := cache.values
	cache.mu.Unlock()

	return darkstats
}

// Collect will collect darkstats metrics locally and fill cache.values with latest data
func Collect(ctx context.Context) error {
	if cache.darkstatAddr == "" {
		return fmt.Errorf("Darkstat address is empty")
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	b, err := prometheus.Scrape(cache.darkstatAddr)
	if err != nil {
		return err
	}

	var metrics []Metric
	for _, v := range b {
		if v.Name == "hostprotoport_bytes_total" {
			for _, m := range v.Metrics {
				metric := m.(prom2json.Metric)
				val, err := strconv.ParseFloat(metric.Value, 64)
				if err != nil {
					log.Errorf("Failed to parse 'hostprotoport_bytes_total' value: %v", err)
					continue
				}
				metrics = append(metrics, Metric{
					Protocol:  metric.Labels["proto"],
					Name:      "",
					Domain:    "",
					Port:      metric.Labels["port"],
					Direction: metric.Labels["dir"],
					Bandwidth: val,
				})
			}
		}
	}

	cache.mu.Lock()
	cache.values = metrics
	cache.mu.Unlock()

	return nil
}
