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
	send_bytes = "ebpf_exporter_ipv4_send_bytes"
	recv_bytes = "ebpf_exporter_ipv4_recv_bytes"
	ingress    = "ingress"
	egress     = "egress"
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
	var send_bytes_metric *prom2json.Family
	var recv_bytes_metric *prom2json.Family
	for _, v := range ebpfScrape {
		if v.Name == send_bytes {
			send_bytes_metric = v
		}
		if v.Name == recv_bytes {
			recv_bytes_metric = v
		}
		if send_bytes_metric != nil && recv_bytes_metric != nil {
			break
		}
	}
	if send_bytes_metric == nil {
		return fmt.Errorf("Metric %v doesn't exist", send_bytes)
	}
	if recv_bytes_metric == nil {
		return fmt.Errorf("Metric %v doesn't exist", recv_bytes)
	}

	sendHostBytes, err := toHostMetrics(send_bytes_metric, egress)
	if err != nil {
		log.Errorf("Conversion to host metric failed for %v, err: %v", send_bytes, err)
	}
	recvHostBytes, err := toHostMetrics(recv_bytes_metric, ingress)
	if err != nil {
		log.Errorf("Conversion to host metric failed for %v, err: %v", recv_bytes, err)
	}

	singleton.mu.Lock()
	singleton.hosts = append(sendHostBytes, recvHostBytes...)
	singleton.mu.Unlock()

	log.Debugf("taskebpf.Collect retrieved %v metrics", len(sendHostBytes)+len(recvHostBytes))
	log.Debugf("taskebpf.Collect process took %v", time.Since(startTime))
	return nil
}

// toHostMetrics converts ebpf metrics into planet explorer prometheus metrics.
func toHostMetrics(bytes_metric *prom2json.Family, direction string) ([]Metric, error) {
	var hosts []Metric
	inventoryHosts := inventory.Get()

	localAddr, err := network.DefaultLocalAddr()
	if err != nil {
		return nil, err
	}

	// To label source traffic that we need to build dependency graph.
	localHostgroup := localAddr.String()
	localDomain := localAddr.String()
	localInventory, ok := inventoryHosts[localAddr.String()]
	if ok {
		localHostgroup = localInventory.Hostgroup
		localDomain = localInventory.Domain
	}
	log.Debugf("Local address doesn't exist in the inventory: %v", localAddr.String())

	for _, m := range bytes_metric.Metrics {
		metric := m.(prom2json.Metric)
		destIp := net.ParseIP(metric.Labels["daddr"])

		if destIp.Equal(localAddr) || destIp.Equal(nil) {
			continue
		}

		inventoryHostInfo := inventoryHosts[metric.Labels["daddr"]]

		bandwidth, err := strconv.ParseFloat(metric.Value, 64)
		if err != nil {
			log.Errorf("Failed to parse 'bytes_metric' value: %v", err)
			continue
		}

		hosts = append(hosts, Metric{
			LocalHostgroup:  localHostgroup,
			RemoteHostgroup: inventoryHostInfo.Hostgroup,
			RemoteIPAddr:    metric.Labels["daddr"],
			LocalDomain:     localDomain,
			RemoteDomain:    inventoryHostInfo.Domain,
			Direction:       direction,
			Bandwidth:       bandwidth,
		})
	}
	return hosts, nil
}
