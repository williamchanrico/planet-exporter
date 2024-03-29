// Copyright 2021 - williamchanrico@gmail.com
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package darkstat

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"planet-exporter/collector/task/inventory"
	"planet-exporter/pkg/network"
	"planet-exporter/pkg/prometheus"

	"github.com/prometheus/prom2json"
	log "github.com/sirupsen/logrus"
)

// task that queries darkstat metrics and aggregates them into usable planet metrics.
type task struct {
	enabled          bool
	darkstatAddr     string
	prometheusClient *prometheus.Client

	hosts []Metric
	mu    sync.Mutex
}

var (
	once      sync.Once
	singleton task
)

func init() {
	httpTransport := &http.Transport{ // nolint:exhaustivestruct
		DialContext: (&net.Dialer{ // nolint:exhaustivestruct
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: true}, // nolint:gosec,exhaustivestruct
		ExpectContinueTimeout: 1 * time.Second,
	}

	singleton = task{
		enabled:          false,
		hosts:            []Metric{},
		mu:               sync.Mutex{},
		prometheusClient: prometheus.New(httpTransport),
		darkstatAddr:     "",
	}
}

// InitTask initial states.
func InitTask(ctx context.Context, enabled bool, darkstatAddr string) {
	once.Do(func() {
		singleton.enabled = enabled
		singleton.darkstatAddr = darkstatAddr
	})
}

// Metric contains values needed for planet metrics.
type Metric struct {
	Direction       string // ingress or egress
	LocalHostgroup  string // e.g. hostgroup
	RemoteHostgroup string
	RemoteIPAddr    string
	LocalDomain     string // e.g. consul domain
	RemoteDomain    string
	Bandwidth       float64
}

// Get returns latest metrics from singleton.
func Get() []Metric {
	singleton.mu.Lock()
	hosts := singleton.hosts
	singleton.mu.Unlock()

	return hosts
}

var (
	// ErrHostBytesTotalMetricsNotFound metrics host_bytes_total not found.
	ErrHostBytesTotalMetricsNotFound = fmt.Errorf("metric host_bytes_total not found")
	// ErrEmptyDarkstatAddr empty darkstat address.
	ErrEmptyDarkstatAddr = fmt.Errorf("darkstat address is empty")
)

// Collect will process darkstats metrics locally and fill singleton with latest data.
func Collect(ctx context.Context) error {
	if !singleton.enabled {
		return nil
	}

	if singleton.darkstatAddr == "" {
		return ErrEmptyDarkstatAddr
	}

	startTime := time.Now()

	ctxCollect, ctxCollectCancel := context.WithCancel(ctx)
	defer ctxCollectCancel()

	// Scrape darkstat prometheus endpoint for host_bytes_total
	var darkstatHostBytesTotalMetric *prom2json.Family
	darkstatScrape, err := singleton.prometheusClient.Scrape(ctxCollect, singleton.darkstatAddr)
	if err != nil {
		return fmt.Errorf("error on darkstat metrics scrape: %w", err)
	}
	for _, v := range darkstatScrape {
		if v.Name == "host_bytes_total" {
			darkstatHostBytesTotalMetric = v

			break
		}
	}
	if darkstatHostBytesTotalMetric == nil {
		return ErrHostBytesTotalMetricsNotFound
	}

	// Extract relevant data out of host_bytes_total
	hosts, err := toHostMetrics(darkstatHostBytesTotalMetric)
	if err != nil {
		return err
	}

	singleton.mu.Lock()
	singleton.hosts = hosts
	singleton.mu.Unlock()

	log.Debugf("taskdarkstat.Collect retrieved %v downstreams metrics", len(hosts))
	log.Debugf("taskdarkstat.Collect process took %v", time.Since(startTime))

	return nil
}

// toHostMetrics converts darkstatHostBytesTotal metrics into planet explorer prometheus metrics.
func toHostMetrics(darkstatHostBytesTotal *prom2json.Family) ([]Metric, error) {
	hosts := []Metric{}

	inventoryHosts := inventory.Get()

	localAddr, err := network.LocalIP()
	if err != nil {
		return nil, fmt.Errorf("error getting local IP address: %w", err)
	}
	// To label source traffic that we need to build dependency graph
	localHostgroup := localAddr.String()
	localDomain := localAddr.String()
	localInventory, ok := inventoryHosts.GetHost(localAddr.String())
	if ok {
		localHostgroup = localInventory.Hostgroup
		localDomain = localInventory.Domain
	} else {
		log.Warnf("Local address don't exist in inventory: %v", localAddr.String())
	}

	for _, m := range darkstatHostBytesTotal.Metrics {
		metric, ok := m.(prom2json.Metric)
		if !ok {
			log.Warnf("Failed to parse darkstat host_bytes_total metrics: %v", m)

			continue
		}

		// Skip its own IP.
		// We're not interested in traffic coming from and going to itself.
		remoteIP := net.ParseIP(metric.Labels["ip"])
		if remoteIP.Equal(nil) || remoteIP.Equal(localAddr) {
			continue
		}

		remoteInventoryHost, _ := inventoryHosts.GetHost(metric.Labels["ip"])

		bandwidth, err := strconv.ParseFloat(metric.Value, 64)
		if err != nil {
			log.Errorf("Failed to parse 'host_bytes_total' value: %v", err)

			continue
		}

		direction := ""
		// Reversed from netfilter perspective
		switch metric.Labels["dir"] {
		case "out":
			direction = "ingress"
		case "in":
			direction = "egress"
		}

		hosts = append(hosts, Metric{
			LocalHostgroup:  localHostgroup,
			RemoteHostgroup: remoteInventoryHost.Hostgroup,
			RemoteIPAddr:    metric.Labels["ip"],
			LocalDomain:     localDomain,
			RemoteDomain:    remoteInventoryHost.Domain,
			Direction:       direction,
			Bandwidth:       bandwidth,
		})
	}

	return hosts, nil
}
