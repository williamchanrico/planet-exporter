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
	"errors"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

const namespace = "planet"

var (
	collectorFactories = make(map[string]func() (Collector, error))

	scrapeDurationDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "scrape", "collector_duration_seconds"),
		"planet_exporter: Duration of a collector scrape.",
		[]string{"collector"},
		nil,
	)
	scrapeSuccessDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "scrape", "collector_success"),
		"planet_exporter: Whether a collector succeeded.",
		[]string{"collector"},
		nil,
	)
)

// PlanetCollector contains our planetary collectors
type PlanetCollector struct {
	Collectors map[string]Collector
}

// Collector interface for new collectors to implement
type Collector interface {
	Update(ch chan<- prometheus.Metric) error
}

// NewPlanetCollector returns new planet collector
func NewPlanetCollector() (*PlanetCollector, error) {
	collectors := make(map[string]Collector)
	for collectorName, factory := range collectorFactories {
		col, err := factory()
		if err != nil {
			return nil, err
		}
		collectors[collectorName] = col
	}

	return &PlanetCollector{
		Collectors: collectors,
	}, nil
}

// Describe implements prometheus.Collector interface.
func (n PlanetCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- scrapeDurationDesc
	ch <- scrapeSuccessDesc
}

// Collect impelements prometheus.Collector interface
func (p PlanetCollector) Collect(ch chan<- prometheus.Metric) {
	wg := sync.WaitGroup{}
	wg.Add(len(p.Collectors))

	for name, collector := range p.Collectors {
		go func(name string, collector Collector) {
			collectorExec(name, collector, ch)

			wg.Done()
		}(name, collector)
	}

	wg.Wait()
}

// ErrNoData returned when collector found no data to collect
var ErrNoData = errors.New("Collector did not find any data")

func collectorExec(name string, c Collector, ch chan<- prometheus.Metric) {
	var success float64

	start := time.Now()
	err := c.Update(ch)
	duration := time.Since(start)
	if err != nil {
		if err == ErrNoData {
			log.Debugf("collector returned no data (name: %v, duration_seconds: %v): %v", name, duration.Seconds(), err)
		} else {
			log.Errorf("collector failed (name: %v, duration_seconds: %v): %v", name, duration.Seconds(), err)
		}
		success = 0
	} else {
		log.Debugf("collector succeeded (name: %v, duration_seconds: %v)", name, duration.Seconds())
		success = 1
	}

	ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, duration.Seconds(), name)
	ch <- prometheus.MustNewConstMetric(scrapeSuccessDesc, prometheus.GaugeValue, success, name)
}

// registerCollector adds the collector to collectorFactories
func registerCollector(name string, factory func() (Collector, error)) {
	collectorFactories[name] = factory
}
