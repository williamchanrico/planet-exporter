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
	"net/http"
	"os"
	"time"

	"planet-exporter/cmd/planet-federator/internal"
	federator "planet-exporter/federator"
	bigqueryFederator "planet-exporter/federator/bigquery"
	influxdbFederator "planet-exporter/federator/influxdb"
	"planet-exporter/prometheus"

	"cloud.google.com/go/bigquery"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	influxdb2domain "github.com/influxdata/influxdb-client-go/v2/domain"
	promapi "github.com/prometheus/client_golang/api"
	log "github.com/sirupsen/logrus"
)

var version string

func main() {
	var err error
	var config internal.Config

	// cronJobTimeOffsetDuration allows federator to go back in time. For example,
	// set '-10h30m' to tell federator to offset query time to 10 hours 30 minutes ago.
	//
	// This is useful when we want to integrate federator to existing Prometheus setup.
	// TODO: Allows running multiple jobs for federator to catch up faster.
	var cronJobTimeOffsetDuration string

	var showVersionAndExit bool

	const (
		defaultInfluxBatchSize      = 20
		defaultCronJobTimeoutSecond = 30
	)

	// Main
	flag.StringVar(&config.CronJobSchedule, "cron-job-schedule", "*/30 * * * * *", "Cron jobs schedule (Quartz: s m h dom mo dow y) to pre-process planet-exporter metrics")
	flag.IntVar(&config.CronJobTimeoutSecond, "cron-job-timeout-second", defaultCronJobTimeoutSecond, "Timeout per federator job in second")
	flag.StringVar(&cronJobTimeOffsetDuration, "cron-job-time-offset", "0s", "Cron jobs time offset. (e.g. '-1h5m' to query data from 1 hour 5 minutes ago)")
	flag.StringVar(&config.LogLevel, "log-level", "info", "Log level")
	flag.BoolVar(&config.LogDisableTimestamp, "log-disable-timestamp", false, "Disable timestamp on logger")
	flag.BoolVar(&config.LogDisableColors, "log-disable-colors", false, "Disable colors on logger")
	flag.BoolVar(&showVersionAndExit, "version", false, "Show version and exit")

	// Prometheus
	flag.StringVar(&config.PrometheusAddr, "prometheus-addr", "http://127.0.0.1:9090/", "Prometheus address containing planet-exporter metrics")

	//
	// Federator backends
	//

	// Backend: Influxdb (default)
	flag.StringVar(&config.InfluxdbAddr, "influxdb-addr", "http://127.0.0.1:8086", "Target Influxdb HTTP Address to store pre-processed planet-exporter data")
	flag.StringVar(&config.InfluxdbToken, "influxdb-token", "", "Target Influxdb token")
	flag.StringVar(&config.InfluxdbOrg, "influxdb-org", "mothership", "Influxdb organization")
	flag.StringVar(&config.InfluxdbBucket, "influxdb-bucket", "mothership", "Influxdb bucket")
	flag.IntVar(&config.InfluxdbBatchSize, "influxdb-batch-size", defaultInfluxBatchSize, "Influxdb batch size")

	// Backend: BigQuery
	// If bq-project-id is set, Federator will use Bigquery backend over Influxdb.
	// TODO: Support a more flexible approach to multiple Federator backends.
	flag.StringVar(&config.BigqueryProjectID, "bq-project-id", "", "BQ Project ID for target dataset")
	// We assume the datasets/tables live in the same GCP Project
	flag.StringVar(&config.BigqueryTrafficTable.DatasetID, "bq-traffic-dataset-id", "", "BQ Dataset ID for traffic table")
	flag.StringVar(&config.BigqueryTrafficTable.TableID, "bq-traffic-table-id", "", "BQ Table ID for traffic table")
	flag.StringVar(&config.BigqueryDependencyTable.DatasetID, "bq-dependency-dataset-id", "", "BQ Dataset ID for dependency table")
	flag.StringVar(&config.BigqueryDependencyTable.TableID, "bq-dependency-table-id", "", "BQ Table ID for dependency table")

	flag.Parse()

	if showVersionAndExit {
		fmt.Println("planet-federator", version) // nolint:forbidigo
		os.Exit(0)
	}

	config.CronJobTimeOffset, err = time.ParseDuration(cronJobTimeOffsetDuration)
	if err != nil {
		log.Fatalf("Error parsing cron-job-time-offset-minute: %v", err)
	}

	log.SetFormatter(&log.TextFormatter{ // nolint:exhaustivestruct
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
		Address:      config.PrometheusAddr,
		RoundTripper: http.DefaultTransport,
	})
	if err != nil {
		log.Fatalf("Error initializing Prometheus client for addr %v: %v", config.PrometheusAddr, err)
	}

	log.Info("Initialize Prometheus service")
	prometheusSvc := prometheus.New(promapiClient)

	log.Info("Initialize Federator service")
	var federatorBackend federator.Backend
	// Prefer BigQuery backend whenever it's set
	if config.BigqueryProjectID != "" {
		log.Info("Initialize Bigquery client")
		bqClient, err := bigquery.NewClient(ctx, config.BigqueryProjectID)
		if err != nil {
			log.Fatalf("Error initializing BigQuery client for GCP Project %v: %v", config.BigqueryProjectID, err)
		}

		federatorBackend = bigqueryFederator.New(bqClient, config.BigqueryTrafficTable, config.BigqueryDependencyTable)

	} else { // Default Influxdb backend
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

		federatorBackend = influxdbFederator.New(influxdbClient, config.InfluxdbOrg, config.InfluxdbBucket)
	}
	federatorSvc := federator.New(federatorBackend)

	log.Info("Initialize main service")
	svc := internal.New(config, federatorSvc, prometheusSvc)
	if err := svc.Run(ctx); err != nil {
		log.Errorf("Main service exit with error: %v", err)
		os.Exit(1) // nolint:gocritic
	}

	log.Info("Main service exit successfully")
}
