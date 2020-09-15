// Copyright 2020 - williamchanrico@gmail.com
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
	"os"
	"planet-exporter/collector/task/inventory"

	"github.com/prometheus/client_golang/prometheus"
)

type hostmetaCollector struct {
	hostname *prometheus.Desc
}

func init() {
	registerCollector("hostmeta", NewHostmetaCollector)
}

func NewHostmetaCollector() (Collector, error) {
	return &hostmetaCollector{
		hostname: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "hostname"),
			"Hostname of the collected machine",
			[]string{"local_hostgroup", "hostname", "domain", "ip"}, nil,
		),
	}, nil
}

func (c hostmetaCollector) Update(ch chan<- prometheus.Metric) error {
	hostname, err := os.Hostname()
	if err != nil {
		// Kernel is probably drunk
		return err
	}
	localInventory := inventory.GetLocalInventory()

	ch <- prometheus.MustNewConstMetric(c.hostname, prometheus.GaugeValue, 1,
		localInventory.Hostgroup, hostname, localInventory.Domain, localInventory.IPAddress)
	return nil
}
