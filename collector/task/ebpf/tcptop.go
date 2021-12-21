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
	"planet-exporter/collector/task/inventory"
	"planet-exporter/pkg/network"
	"planet-exporter/pkg/prometheus"
	"strconv"
	"sync"
	"time"

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
	httpTransport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
		ExpectContinueTimeout: 1 * time.Second,
	}

	singleton = task{
		enabled:          false,
		hosts:            []Metric{},
		mu:               sync.Mutex{},
		prometheusClient: prometheus.New(httpTransport),
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

// Collect will process ebpf metrics locally and fill singleton with latest data.
func Collect(ctx context.Context) error {
	if !singleton.enabled {
		return nil
	}

	if singleton.ebpfAddr == "" {
		return fmt.Errorf("eBPF address is empty")
	}

	startTime := time.Now()

	ctxCollect, ctxCollectCancel := context.WithCancel(ctx)
	defer ctxCollectCancel()

	// Scrape ebpf prometheus endpoint for send_bytes_metric and recv_bytes_metric.
	ebpfScrape, err := singleton.prometheusClient.Scrape(ctxCollect, singleton.ebpfAddr)
	if err != nil {
		return err
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
		return fmt.Errorf("Metric %v doesn't exist", sendBytes)
	}
	if recvBytesMetric == nil {
		return fmt.Errorf("Metric %v doesn't exist", recvBytes)
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
	singleton.hosts = append(sendHostBytes, recvHostBytes...)
	singleton.mu.Unlock()

	log.Debugf("taskebpf.Collect retrieved %v metrics", len(sendHostBytes)+len(recvHostBytes))
	log.Debugf("taskebpf.Collect process took %v", time.Since(startTime))
	return nil
}

// toHostMetrics converts ebpf metrics into planet explorer prometheus metrics.
func toHostMetrics(bytesMetric *prom2json.Family, direction string) ([]Metric, error) {
	var hosts []Metric
	inventoryHosts := inventory.Get()

	localAddr, err := network.DefaultLocalAddr()
	if err != nil {
		return nil, err
	}

	// To label source traffic that we need to build dependency graph.
	localHostgroup := localAddr.String()
	localDomain := localAddr.String()
	localInventory, ok := inventoryHosts.GetHost(localAddr.String())
	if ok {
		localHostgroup = localInventory.Hostgroup
		localDomain = localInventory.Domain
	} else {
		log.Warnf("Local address doesn't exist in the inventory: %v", localAddr.String())
	}

	for _, m := range bytesMetric.Metrics {
		metric := m.(prom2json.Metric)

		// Skip its own IP.
		// We're not interested in traffic coming from and going to itself.
		remoteIP := net.ParseIP(metric.Labels["daddr"])
		if remoteIP.Equal(nil) || remoteIP.Equal(localAddr) {
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
