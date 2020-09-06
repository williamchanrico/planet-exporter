package collector

import (
	"planet-exporter/collector/task/darkstat"

	"github.com/prometheus/client_golang/prometheus"
)

type networkDependencyCollector struct {
	upstream *prometheus.Desc
}

func init() {
	registerCollector("network_dependency", NewNetworkDependencyCollector)
}

func NewNetworkDependencyCollector() (Collector, error) {
	return &networkDependencyCollector{
		upstream: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "upstream"),
			"Upstream dependency of this machine",
			[]string{"protocol", "name", "domain", "port", "direction"}, nil,
		),
	}, nil
}

func (c networkDependencyCollector) Update(ch chan<- prometheus.Metric) error {
	darkstatMetrics := darkstat.Get()

	for _, m := range darkstatMetrics {
		ch <- prometheus.MustNewConstMetric(c.upstream, prometheus.GaugeValue, m.Bandwidth,
			m.Protocol, m.Name, m.Domain, m.Port, m.Direction)
	}

	return nil
}
