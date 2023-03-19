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

package internal

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	federatorquery "planet-exporter/federator/influxdb/query"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"

	influxdb1 "github.com/influxdata/influxdb1-client/v2"
	cron "github.com/robfig/cron/v3"
	log "github.com/sirupsen/logrus"
)

// Config contains main service config options.
type Config struct {
	// Main config
	// CronJobSchedule schedule using cron format used by the Quartz Scheduler
	// 1. Seconds
	// 2. Minutes
	// 3. Hours
	// 4. Day-of-Month
	// 5. Month
	// 6. Day-of-Week
	// 7. Year (optional field)
	CronJobScheduleTrafficJob    string
	CronJobScheduleDependencyJob string
	CronJobTimeoutSecond         int
	// CronJobTimeOffset all cron job start time (e.g. '-5m' will query data from 5 minutes ago)
	CronJobTimeOffset   time.Duration
	LogLevel            string
	LogDisableTimestamp bool
	LogDisableColors    bool

	InfluxdbAddr     string
	InfluxdbUsername string
	InfluxdbPassword string
	InfluxdbDatabase string

	BigqueryProjectID         string
	BigqueryDatasetID         string
	BigqueryTrafficTableID    string
	BigqueryDependencyTableID string
}

// Service contains main service dependency.
type Service struct {
	Config Config
	// Source data from Federator InfluxDB
	queryInfluxDB *federatorquery.Client
	// Destination backend storage
	storeBackend backend
}

// New service.
func New(config Config, influxdbClient influxdb1.Client, bqClient *bigquery.Client) Service {
	backend := newBackend(config, bqClient)
	return Service{
		Config:        config,
		queryInfluxDB: federatorquery.New(influxdbClient, config.InfluxdbDatabase),
		storeBackend:  backend,
	}
}

// Run main service.
func (s Service) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	log.Info("Start Cron scheduler")
	cronScheduler := cron.New(cron.WithSeconds())
	_, err := cronScheduler.AddFunc(s.Config.CronJobScheduleTrafficJob, s.TrafficBandwidthJobFunc)
	if err != nil {
		return fmt.Errorf("error adding TrafficBandwidthJobFunc function to Cron scheduler: %w", err)
	}
	_, err = cronScheduler.AddFunc(s.Config.CronJobScheduleDependencyJob, s.DependencyDataJobFunc)
	if err != nil {
		return fmt.Errorf("error adding DependencyDataJobFunc function to Cron scheduler: %w", err)
	}
	cronScheduler.Start()

	// Capture signals and graceful exit mechanism
	stopChan := make(chan struct{})
	go func() {
		signals := make(chan os.Signal, 2)
		signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
		select {
		case <-signals:
			log.Info("Detected stop signal!")

			log.Info("Flush any pending actions")

			log.Info("Stop Cron scheduler")
			cronStopCtx := cronScheduler.Stop()
			cronStopTimeoutTimer := time.NewTimer(time.Duration(s.Config.CronJobTimeoutSecond) * time.Second)
			select {
			case <-cronStopCtx.Done():
			case <-cronStopTimeoutTimer.C:
				log.Warn("Timeout waiting for running Cron jobs to stop!")
			}

			log.Info("Graceful stop completed")

		case <-ctx.Done():
		}

		close(stopChan)
	}()

	<-stopChan

	return nil
}

// getCronJobStartTime returns the time for cron job starting point.
func (s Service) getCronJobStartTime() time.Time {
	// We want to offset the query time by the specified offset
	return time.Now().Add(s.Config.CronJobTimeOffset)
}

// getCronJobDuration returns the duration since the cron job was started.
func (s Service) getCronJobDuration(startTime time.Time) time.Duration {
	// We want to offset the query time by the specified offset
	return time.Now().Add(s.Config.CronJobTimeOffset).Sub(startTime)
}

// TrafficBandwidthJobFunc queries traffic bandwidth (planet-federator) data from InfluxDB and stores
// them in Backend (i.e. BigQuery).
func (s Service) TrafficBandwidthJobFunc() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.Config.CronJobTimeoutSecond)*time.Second)
	defer cancel()

	jobStartTime := s.getCronJobStartTime()
	log.Debugf("A job started: %v", jobStartTime)

	trafficPeers, err := s.queryInfluxDB.QueryFederatorTraffic(ctx)
	if err != nil {
		log.Errorf("error querying traffic data from influxdb: %v", err)
	}

	trafficTableData := []TrafficTableData{}
	for _, trafficPeer := range trafficPeers {
		localAddress := bigquery.NullString{}
		if trafficPeer.LocalHostgroupAddress != "" {
			localAddress.StringVal = trafficPeer.LocalHostgroupAddress
			localAddress.Valid = true
		}
		remoteAddress := bigquery.NullString{}
		if trafficPeer.RemoteHostgroupAddress != "" {
			remoteAddress.StringVal = trafficPeer.RemoteHostgroupAddress
			remoteAddress.Valid = true
		}
		trafficTableData = append(trafficTableData, TrafficTableData{
			InventoryDate:             civil.DateTimeOf(jobStartTime),
			TrafficDirection:          trafficPeer.TrafficDirection,
			LocalHostgroup:            trafficPeer.LocalHostgroup,
			LocalHostgroupAddress:     localAddress,
			RemoteHostgroup:           trafficPeer.RemoteHostgroup,
			RemoteHostgroupAddress:    remoteAddress,
			TrafficBandwidthBitsMin1h: trafficPeer.TrafficBandwidthBitsMin1h,
			TrafficBandwidthBitsMax1h: trafficPeer.TrafficBandwidthBitsMax1h,
			TrafficBandwidthBitsAvg1h: trafficPeer.TrafficBandwidthBitsAvg1h,
		})
	}

	err = s.storeBackend.InsertTrafficBandwidthData(ctx, trafficTableData)
	if err != nil {
		log.Errorf("error InsertTrafficBandwidthData: %v", err)
	}

	log.Infof("Traffic Bandwidth Job took: %v", s.getCronJobDuration(jobStartTime))
}

// DependencyDataJobFunc queries upstream & downstream dependencies (planet-federator) data from InfluxDB and stores
// them in Backend (i.e. BigQuery).
func (s Service) DependencyDataJobFunc() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.Config.CronJobTimeoutSecond)*time.Second)
	defer cancel()

	jobStartTime := s.getCronJobStartTime()
	log.Debugf("A job started: %v", jobStartTime)

	dependencies, err := s.queryInfluxDB.QueryFederatorDependencyLast7d(ctx)
	if err != nil {
		log.Errorf("error querying dependency data from influxdb: %v", err)
	}

	dependencyTableData := []DependencyData{}
	for _, dependency := range dependencies {
		localProcessName := bigquery.NullString{}
		if dependency.LocalHostgroupProcessName != "" {
			localProcessName.StringVal = dependency.LocalHostgroupProcessName
			localProcessName.Valid = true
		}
		localAddress := bigquery.NullString{}
		if dependency.LocalHostgroupAddress != "" {
			localAddress.StringVal = dependency.LocalHostgroupAddress
			localAddress.Valid = true
		}
		localPort := bigquery.NullString{}
		if dependency.LocalHostgroupAddressPort != "" {
			localPort.StringVal = dependency.LocalHostgroupAddressPort
			localPort.Valid = true
		}

		remoteAddress := bigquery.NullString{}
		if dependency.RemoteHostgroupAddress != "" {
			remoteAddress.StringVal = dependency.RemoteHostgroupAddress
			remoteAddress.Valid = true
		}
		remotePort := bigquery.NullString{}
		if dependency.RemoteHostgroupAddressPort != "" {
			remotePort.StringVal = dependency.RemoteHostgroupAddressPort
			remotePort.Valid = true
		}

		dependencyTableData = append(dependencyTableData, DependencyData{
			InventoryDate: civil.DateTimeOf(jobStartTime),

			DependencyDirection:       dependency.Direction,
			Protocol:                  dependency.Protocol,
			LocalHostgroupProcessName: localProcessName,

			// This is null for an upstream dependency data
			LocalHostgroupAddressPort: localPort,

			LocalHostgroup:        dependency.LocalHostgroup,
			LocalHostgroupAddress: localAddress,

			RemoteHostgroup:        dependency.RemoteHostgroup,
			RemoteHostgroupAddress: remoteAddress,

			RemoteHostgroupAddressPort: remotePort,
		})
	}

	err = s.storeBackend.InsertDependencyData(ctx, dependencyTableData)
	if err != nil {
		log.Errorf("error InsertDependencyData: %v", err)
	}

	log.Infof("Dependency Job took: %v", s.getCronJobDuration(jobStartTime))
}
