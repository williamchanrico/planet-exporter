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

	"planet-exporter/cmd/planet-federator/internal"
	federator "planet-exporter/federator"
	influxdbFederator "planet-exporter/federator/influxdb"
	"planet-exporter/prometheus"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	influxdb2domain "github.com/influxdata/influxdb-client-go/v2/domain"
	promapi "github.com/prometheus/client_golang/api"
	log "github.com/sirupsen/logrus"
)

var (
	version            string
	showVersionAndExit bool
)

func main() {
	var config internal.Config

	// Main
	flag.StringVar(&config.CronJobSchedule, "cron-job-schedule", "*/30 * * * * *", "Cron jobs schedule to pre-process planet-exporter metrics into federator backend")
	flag.IntVar(&config.CronJobTimeoutSecond, "cron-job-timeout-second", 30, "Timeout per federator job in second")
	flag.StringVar(&config.LogLevel, "log-level", "info", "Log level")
	flag.BoolVar(&config.LogDisableTimestamp, "log-disable-timestamp", false, "Disable timestamp on logger")
	flag.BoolVar(&config.LogDisableColors, "log-disable-colors", false, "Disable colors on logger")
	flag.BoolVar(&showVersionAndExit, "version", false, "Show version and exit")

	// Influxdb
	flag.StringVar(&config.InfluxdbAddr, "influxdb-addr", "http://127.0.0.1:8086", "Target Influxdb HTTP Address to store pre-processed planet-exporter data")
	flag.StringVar(&config.InfluxdbToken, "influxdb-token", "", "Target Influxdb token")
	flag.StringVar(&config.InfluxdbOrg, "influxdb-org", "mothership", "Influxdb organization")
	flag.StringVar(&config.InfluxdbBucket, "influxdb-bucket", "mothership", "Influxdb bucket")
	flag.IntVar(&config.InfluxdbBatchSize, "influxdb-batch-size", 20, "Influxdb batch size")

	// Prometheus
	flag.StringVar(&config.PrometheusAddr, "prometheus-addr", "http://127.0.0.1:9090/", "Prometheus address containing planet-exporter metrics")

	flag.Parse()

	if showVersionAndExit {
		fmt.Printf("planet-federator %v\n", version)
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

	log.Infof("Planet Federator %v", version)
	log.Infof("Initialize log with level %v", config.LogLevel)

	ctx := context.Background()

	log.Info("Initialize Prometheus API client")
	promapiClient, err := promapi.NewClient(promapi.Config{
		Address: config.PrometheusAddr,
	})
	if err != nil {
		log.Fatalf("Error initializing Prometheus client for addr %v: %v", config.PrometheusAddr, err)
	}

	log.Info("Initialize Influxdb client")
	influxdbClient := influxdb2.NewClient(config.InfluxdbAddr, config.InfluxdbToken)
	influxdbHealth, err := influxdbClient.Health(ctx)
	if err != nil {
		log.Fatalf("Target Influxdb (%v) health-check error: %v", config.InfluxdbAddr, err)
	}
	if influxdbHealth.Status != influxdb2domain.HealthCheckStatusPass {
		log.Fatalf("Target Influxdb (%v) is unhealthy: %v", config.InfluxdbAddr, err)
	}
	defer influxdbClient.Close()

	log.Info("Initialize Prometheus service")
	prometheusSvc := prometheus.New(promapiClient)

	log.Info("Initialize Federator service")
	federatorBackend := influxdbFederator.New(influxdbClient, config.InfluxdbOrg, config.InfluxdbBucket)
	federatorSvc := federator.New(federatorBackend)

	log.Info("Initialize main service")
	svc := internal.New(config, federatorSvc, prometheusSvc)
	if err := svc.Run(ctx); err != nil {
		log.Fatalf("Main service exit with error: %v", err)
	}

	log.Info("Main service exit successfully")
}
