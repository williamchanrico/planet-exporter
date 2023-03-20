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
	"time"

	"planet-exporter/cmd/planet-federator-influxdb-to-bq/internal"

	"cloud.google.com/go/bigquery"
	influxdb1 "github.com/influxdata/influxdb1-client/v2"
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
		defaultCronJobTimeoutSecond = 300
	)

	// Main
	flag.StringVar(&config.CronJobScheduleTrafficJob, "cron-job-schedule-traffic", "30 0 * * * *", "Cron jobs schedule (Quartz: s m h dom mo dow y) to process federator traffic data")
	flag.StringVar(&config.CronJobScheduleDependencyJob, "cron-job-schedule-dependency", "30 0 11 * * *", "Cron jobs schedule (Quartz: s m h dom mo dow y) to process federator dependency data")
	flag.IntVar(&config.CronJobTimeoutSecond, "cron-job-timeout-second", defaultCronJobTimeoutSecond, "Timeout per federator job in second")
	flag.StringVar(&cronJobTimeOffsetDuration, "cron-job-time-offset", "0s", "Cron jobs time offset. (e.g. '-1h5m' to query data from 1 hour 5 minutes ago)")
	flag.StringVar(&config.LogLevel, "log-level", "info", "Log level")
	flag.BoolVar(&config.LogDisableTimestamp, "log-disable-timestamp", false, "Disable timestamp on logger")
	flag.BoolVar(&config.LogDisableColors, "log-disable-colors", false, "Disable colors on logger")
	flag.BoolVar(&showVersionAndExit, "version", false, "Show version and exit")

	// Source InfluxDB
	flag.StringVar(&config.InfluxdbAddr, "influxdb-addr", "http://127.0.0.1:8086", "Target InfluxDB HTTP Address that stores the pre-processed planet-exporter data")
	flag.StringVar(&config.InfluxdbUsername, "influxdb-username", "", "Target InfluxDB username")
	flag.StringVar(&config.InfluxdbPassword, "influxdb-password", "", "Target InfluxDB password")
	flag.StringVar(&config.InfluxdbDatabase, "influxdb-database", "mothership", "InfluxDB organization")

	// Destination BigQuery
	// We assume the tables live in the same GCP Project and same Dataset
	flag.StringVar(&config.BigqueryProjectID, "bq-project-id", "", "BQ Project ID for target dataset")
	flag.StringVar(&config.BigqueryDatasetID, "bq-dataset-id", "", "BQ Dataset ID for traffic table")
	flag.StringVar(&config.BigqueryTrafficTableID, "bq-traffic-table-id", "planet_exporter_traffic", "BQ Table ID for traffic table")
	flag.StringVar(&config.BigqueryDependencyTableID, "bq-dependency-table-id", "planet_exporter_dependency", "BQ Table ID for dependency table")

	flag.Parse()

	if showVersionAndExit {
		fmt.Println("planet-federator-influxdb-to-bq", version) // nolint:forbidigo
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

	log.Infof("Planet Federator InfluxDB to BQ %v", version)
	log.Infof("Initialize log with level %v", config.LogLevel)

	ctx := context.Background()

	log.Info("Initialize InfluxDB to BQ service")

	log.Info("Initialize Influxdb client")
	influxdbClient, err := influxdb1.NewHTTPClient(influxdb1.HTTPConfig{
		Addr:     config.InfluxdbAddr,
		Username: config.InfluxdbUsername,
		Password: config.InfluxdbPassword,
		Timeout:  time.Second * time.Duration(config.CronJobTimeoutSecond),
	})
	if err != nil {
		fmt.Println("Error creating InfluxDB Client: ", err.Error())
	}
	defer influxdbClient.Close()

	log.Info("Initialize Bigquery client")
	bqClient, err := bigquery.NewClient(ctx, config.BigqueryProjectID)
	if err != nil {
		log.Fatalf("Error initializing BigQuery client for GCP Project %v: %v", config.BigqueryProjectID, err)
	}

	log.Info("Initialize main service")
	svc := internal.New(config, influxdbClient, bqClient)
	if err := svc.Run(ctx); err != nil {
		log.Errorf("Main service exit with error: %v", err)
		os.Exit(1) // nolint:gocritic
	}

	log.Info("Main service exit successfully")
}
