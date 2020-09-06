package internal

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"planet-exporter/collector"
	taskdarkstat "planet-exporter/collector/task/darkstat"
	"planet-exporter/server"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/version"
	log "github.com/sirupsen/logrus"
)

// Config contains main service config options
type Config struct {
	// Main config
	ListenAddress string
	LogLevel      string

	// Collector tasks config
	// TaskInterval between each collection of some expensive data computation, in Duration format (e.g. "7s").
	TaskInterval string
	// DarkstatAddr url for darkstat metrics scrape
	DarkstatAddr string
	// InventoryAddr url for inventory hostgroup mapping table data
	InventoryAddr string
}

// Service contains main service dependency
type Service struct {
	Config Config

	// Collector is prometheus collector that is registered
	Collector *collector.PlanetCollector
}

// New service
func New(config Config, collector *collector.PlanetCollector) Service {
	return Service{
		Config:    config,
		Collector: collector,
	}
}

// Run main service
func (s Service) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Run expensive task in background
	log.Infof("Set task ticker duration to %v", s.Config.TaskInterval)
	interval, err := time.ParseDuration(s.Config.TaskInterval)
	if err != nil {
		return err
	}
	go s.collect(ctx, interval)

	r := prometheus.NewRegistry()
	r.MustRegister(version.NewCollector("planet_exporter"))
	if err := r.Register(s.Collector); err != nil {
		return fmt.Errorf("Failed to register planet collector: %v", err)
	}

	handler := http.NewServeMux()
	handler.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
			<head><title>Node Exporter</title></head>
			<body>
			<h1>Node Exporter</h1>
			<p><a href="/metrics">Metrics</a></p>
			</body>
			</html>`))
	})
	handler.Handle("/metrics", promhttp.HandlerFor(
		prometheus.Gatherers{r},
		promhttp.HandlerOpts{
			ErrorHandling: promhttp.ContinueOnError,
		},
	))
	httpServer := server.New(handler)

	// Capture signals and graceful exit mechanism
	stopChan := make(chan struct{})
	go func() {
		signals := make(chan os.Signal, 2)
		signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
		select {
		case <-signals:
			log.Info("Gracefully stop HTTP server")
			if err := httpServer.Shutdown(ctx); err != nil {
				log.Errorf("Failed to stop http server: %v", err)
			}
		case <-ctx.Done():
		}

		close(stopChan)
	}()

	log.Infof("Start HTTP server on %v", s.Config.ListenAddress)
	if err := httpServer.Serve(s.Config.ListenAddress); err != http.ErrServerClosed {
		return err
	}

	<-stopChan

	return nil
}

// collect runs collector tasks that are expensive to compute on-the-fly
func (s Service) collect(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Darkstat query
			err := taskdarkstat.Collect(ctx, s.Config.DarkstatAddr)
			if err != nil {
				log.Errorf("Darkstat collect failed: %v", err)
			}

			// Inventory query
			// err = taskinventory.Collect(ctx, s.Config.InventoryAddr)
			// if err != nil {
			//     log.Errorf("Inventory collect failed: %v", err)
			// }
		case <-ctx.Done():
			return
		}
	}
}

// // runCollectors will trigger Metrics() from initialized exporters at a given interval
// func (s Service) runCollectors(ctx context.Context) error {
//     if s.Tasks == nil {
//         log.Debugf("No crons was configured")
//         return nil
//     }

//     ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
//     defer cancel()

//     for _, task := range s.Tasks {
//         log.Debugf("Collect from %v exporter", task.Name())

//         if err := task.Run(); err != nil {
//             log.Errorf("Failed to collect %v: %v", task.Name(), err)
//             continue
//         }
//     }

//     return nil
// }
