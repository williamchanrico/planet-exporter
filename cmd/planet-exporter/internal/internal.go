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

package internal

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"net/http/pprof"

	"planet-exporter/collector"
	taskdarkstat "planet-exporter/collector/task/darkstat"
	taskinventory "planet-exporter/collector/task/inventory"
	tasksocketstat "planet-exporter/collector/task/socketstat"
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

	// TaskInterval between each collection of some expensive data computation
	// in Duration format (e.g. "7s").
	TaskInterval string

	TaskDarkstatEnabled bool
	TaskDarkstatAddr    string // DarkstatAddr url for darkstat metrics scrape

	TaskInventoryEnabled bool
	TaskInventoryAddr    string // InventoryAddr url for inventory hostgroup mapping table data

	TaskSocketstatEnabled bool
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

	// Run collector tasks in background
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
				<head><title>Planet Exporter</title></head>
				<body>
				<h1>Planet Exporter</h1>
				<p><a href="/metrics">Metrics</a></p>
				</body>
			</html>
		`))
	})
	handler.Handle("/metrics", promhttp.HandlerFor(
		prometheus.Gatherers{r},
		promhttp.HandlerOpts{
			ErrorHandling: promhttp.ContinueOnError,
		},
	))
	handler.HandleFunc("/debug/pprof/", pprof.Index)
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

// collect periodically runs all collector tasks that are expensive to compute on-the-fly
func (s Service) collect(ctx context.Context, interval time.Duration) {
	inventoryTicker := time.NewTicker(interval * 25)
	defaultTicker := time.NewTicker(interval)
	defer inventoryTicker.Stop()
	defer defaultTicker.Stop()

	log.Info("Initialize collector tasks")

	log.Infof("Task Darkstat: %v", s.Config.TaskDarkstatEnabled)
	taskdarkstat.InitTask(ctx, s.Config.TaskDarkstatEnabled, s.Config.TaskDarkstatAddr)

	log.Infof("Task Inventory: %v", s.Config.TaskInventoryEnabled)
	taskinventory.InitTask(ctx, s.Config.TaskInventoryEnabled, s.Config.TaskInventoryAddr)

	log.Infof("Task Socketstat: %v", s.Config.TaskSocketstatEnabled)
	tasksocketstat.InitTask(ctx, s.Config.TaskSocketstatEnabled)

	fInventory := func() {
		err := taskinventory.Collect(ctx)
		if err != nil {
			log.Errorf("Inventory collect failed: %v", err)
		}
	}
	fDefault := func() {
		err := taskdarkstat.Collect(ctx)
		if err != nil {
			log.Errorf("Darkstat collect failed: %v", err)
		}
		err = tasksocketstat.Collect(ctx)
		if err != nil {
			log.Errorf("Socketstat collect failed: %v", err)
		}
	}

	// Trigger once
	fInventory()
	fDefault()

	for {
		select {
		case <-inventoryTicker.C:
			log.Debugf("Start inventory collect tick")
			fInventory()

		case <-defaultTicker.C:
			log.Debugf("Start default collect tick")
			fDefault()

		case <-ctx.Done():
			return
		}
	}
}
