/**
 * Copyright 2021
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package ebpf

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

// task that queries ebpf metrics and aggregates them into usable planet metrics.
type task struct {
	enabled          bool
	ebpfAddr         string
	prometheusClient *prometheus.Client

	hosts []Metric
	mu    sync.Mutex
}

var (
	once      sync.Once
	singleton task
)

const (
	sendBytes = "ebpf_exporter_ipv4_send_bytes"
	recvBytes = "ebpf_exporter_ipv4_recv_bytes"
	ingress   = "ingress"
	egress    = "egress"
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
		ebpfAddr:         "",
	}
}

// InitTask initial states.
func InitTask(ctx context.Context, enabled bool, ebpfAddr string) {
	once.Do(func() {
		singleton.enabled = enabled
		singleton.ebpfAddr = ebpfAddr
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
	// ErrMetricsNotFound metrics does not exists.
	ErrMetricsNotFound = fmt.Errorf("metrics does not exists")
	// ErrEmptyEBPFAddr ebpf address is empty.
	ErrEmptyEBPFAddr = fmt.Errorf("ebpf address is empty")
)

// Collect will process ebpf metrics locally and fill singleton with latest data.
// nolint:cyclop
func Collect(ctx context.Context) error {
	if !singleton.enabled {
		return nil
	}

	if singleton.ebpfAddr == "" {
		return ErrEmptyEBPFAddr
	}

	startTime := time.Now()

	ctxCollect, ctxCollectCancel := context.WithCancel(ctx)
	defer ctxCollectCancel()

	// Scrape ebpf prometheus endpoint for send_bytes_metric and recv_bytes_metric.
	ebpfScrape, err := singleton.prometheusClient.Scrape(ctxCollect, singleton.ebpfAddr)
	if err != nil {
		return fmt.Errorf("error on ebpf metrics scrape: %w", err)
	}
	var sendBytesMetric *prom2json.Family
	var recvBytesMetric *prom2json.Family
	for _, v := range ebpfScrape {
		if v.Name == sendBytes {
			sendBytesMetric = v
		}
		if v.Name == recvBytes {
			recvBytesMetric = v
		}
		if sendBytesMetric != nil && recvBytesMetric != nil {
			break
		}
	}
	if sendBytesMetric == nil {
		return ErrMetricsNotFound
	}
	if recvBytesMetric == nil {
		return ErrMetricsNotFound
	}

	sendHostBytes, err := toHostMetrics(sendBytesMetric, egress)
	if err != nil {
		log.Errorf("Conversion to host metric failed for %v, err: %v", sendBytes, err)
	}
	recvHostBytes, err := toHostMetrics(recvBytesMetric, ingress)
	if err != nil {
		log.Errorf("Conversion to host metric failed for %v, err: %v", recvBytes, err)
	}

	singleton.mu.Lock()
	singleton.hosts = append(sendHostBytes, recvHostBytes...) // nolint:gocritic
	singleton.mu.Unlock()

	log.Debugf("taskebpf.Collect retrieved %v metrics", len(sendHostBytes)+len(recvHostBytes))
	log.Debugf("taskebpf.Collect process took %v", time.Since(startTime))

	return nil
}

// toHostMetrics converts ebpf metrics into planet explorer prometheus metrics.
func toHostMetrics(bytesMetric *prom2json.Family, direction string) ([]Metric, error) {
	hosts := []Metric{}
	inventoryHosts := inventory.Get()

	currentIP, err := network.LocalIP()
	if err != nil {
		return nil, fmt.Errorf("error getting local IP address: %w", err)
	}

	// To label source traffic that we need to build dependency graph.
	localHostgroup := currentIP.String()
	localDomain := currentIP.String()
	localInventory, ok := inventoryHosts.GetHost(currentIP.String())
	if ok {
		localHostgroup = localInventory.Hostgroup
		localDomain = localInventory.Domain
	} else {
		log.Warnf("Local address doesn't exist in the inventory: %v", currentIP.String())
	}

	for _, m := range bytesMetric.Metrics {
		metric, ok := m.(prom2json.Metric)
		if !ok {
			log.Warnf("Failed to parse ebpf metrics: %v", m)

			continue
		}

		// Skip its own IP.
		// We're not interested in traffic coming from and going to itself.
		remoteIP := net.ParseIP(metric.Labels["daddr"])
		if remoteIP.Equal(nil) || remoteIP.Equal(currentIP) {
			continue
		}

		remoteInventoryHost, _ := inventoryHosts.GetHost(metric.Labels["daddr"])

		bandwidth, err := strconv.ParseFloat(metric.Value, 64)
		if err != nil {
			log.Errorf("Failed to parse 'bytes_metric' value: %v", err)

			continue
		}

		hosts = append(hosts, Metric{
			LocalHostgroup:  localHostgroup,
			RemoteHostgroup: remoteInventoryHost.Hostgroup,
			RemoteIPAddr:    metric.Labels["daddr"],
			LocalDomain:     localDomain,
			RemoteDomain:    remoteInventoryHost.Domain,
			Direction:       direction,
			Bandwidth:       bandwidth,
		})
	}

	return hosts, nil
}
