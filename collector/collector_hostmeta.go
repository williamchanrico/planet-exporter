package collector

import (
	"os"

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
			[]string{"hostname"}, nil,
		),
	}, nil
}

func (c hostmetaCollector) Update(ch chan<- prometheus.Metric) error {
	hostname, err := os.Hostname()
	if err != nil {
		// Kernel is probably drunk
		return err
	}
	ch <- prometheus.MustNewConstMetric(c.hostname, prometheus.GaugeValue, 1,
		hostname)
	return nil
}
