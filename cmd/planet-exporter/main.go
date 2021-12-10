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

package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"planet-exporter/cmd/planet-exporter/internal"
	"planet-exporter/collector"

	log "github.com/sirupsen/logrus"
)

var (
	version            string
	showVersionAndExit bool
)

func main() {
	var config internal.Config

	// Main
	flag.StringVar(&config.ListenAddress, "listen-address", "0.0.0.0:19100", "Address to which exporter will bind its HTTP interface")
	flag.StringVar(&config.LogLevel, "log-level", "info", "Log level")
	flag.BoolVar(&config.LogDisableTimestamp, "log-disable-timestamp", false, "Disable timestamp on logger")
	flag.BoolVar(&config.LogDisableColors, "log-disable-colors", false, "Disable colors on logger")
	flag.BoolVar(&showVersionAndExit, "version", false, "Show version and exit")

	// Collector tasks
	flag.StringVar(&config.TaskInterval, "task-interval", "7s", "Interval between collection of expensive data into memory")

	flag.BoolVar(&config.TaskSocketstatEnabled, "task-socketstat-enabled", true, "Enable socketstat collector task")

	flag.BoolVar(&config.TaskDarkstatEnabled, "task-darkstat-enabled", false, "Enable darkstat collector task")
	flag.StringVar(&config.TaskDarkstatAddr, "task-darkstat-addr", "", "Darkstat target address")

	flag.BoolVar(&config.TaskEbpfEnabled, "task-ebpf-enabled", false, "Enable Ebpf collector task")
	flag.StringVar(&config.TaskEbpfAddr, "task-ebpf-addr", "http://localhost:9435/metrics", "Ebpf target address")

	flag.BoolVar(&config.TaskInventoryEnabled, "task-inventory-enabled", false, "Enable inventory collector task")
	flag.StringVar(&config.TaskInventoryAddr, "task-inventory-addr", "", "Darkstat target address")

	flag.Parse()

	if showVersionAndExit {
		fmt.Printf("planet-exporter %v\n", version)
		os.Exit(0)
	}

	log.SetFormatter(&log.TextFormatter{
		DisableColors:    config.LogDisableColors,
		DisableTimestamp: config.LogDisableTimestamp,
		FullTimestamp:    true,
	})
	logLevel, err := log.ParseLevel(config.LogLevel)
	if err != nil {
		log.Fatalf("Failed to parse log level: %v", err)
	}
	log.SetLevel(logLevel)

	log.Infof("Planet Exporter %v", version)
	log.Infof("Initialize log with level %v", config.LogLevel)

	ctx := context.Background()

	log.Info("Initialize prometheus collector")
	collector, err := collector.NewPlanetCollector()
	if err != nil {
		log.Fatalf("Failed to initialize planet collector: %v", err)
	}

	log.Info("Initialize main service")
	svc := internal.New(config, collector)
	if err := svc.Run(ctx); err != nil {
		log.Fatalf("Main service exit with error: %v", err)
	}

	log.Info("Main service exit successfully")
}
