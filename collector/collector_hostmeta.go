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

package collector

import (
	"fmt"
	"os"

	"planet-exporter/collector/task/inventory"

	"github.com/prometheus/client_golang/prometheus"
)

// hostmetaCollector on host related metadata.
type hostmetaCollector struct {
	hostname *prometheus.Desc
}

func init() {
	registerCollector("hostmeta", NewHostmetaCollector)
}

// NewHostmetaCollector service.
func NewHostmetaCollector() (Collector, error) {
	return &hostmetaCollector{
		hostname: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "hostname"),
			"Hostname of the collected machine",
			[]string{"local_hostgroup", "hostname", "domain", "ip"}, nil,
		),
	}, nil
}

// Update implements Collector interface.
func (c hostmetaCollector) Update(prometheusMetricsCh chan<- prometheus.Metric) error {
	hostname, err := os.Hostname()
	if err != nil {
		// Kernel is probably drunk
		return fmt.Errorf("error getting hostname: %w", err)
	}
	localInventory := inventory.GetLocalInventory()

	prometheusMetricsCh <- prometheus.MustNewConstMetric(c.hostname, prometheus.GaugeValue, 1,
		localInventory.Hostgroup, hostname, localInventory.Domain, localInventory.IPAddress)

	return nil
}
