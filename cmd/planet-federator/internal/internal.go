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

	"planet-exporter/federator"
	"planet-exporter/prometheus"

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
	CronJobSchedule      string
	CronJobTimeoutSecond int
	// CronJobTimeOffset all cron job start time (e.g. '-5m' will query data from 5 minutes ago)
	CronJobTimeOffset   time.Duration
	LogLevel            string
	LogDisableTimestamp bool
	LogDisableColors    bool

	InfluxdbAddr      string
	InfluxdbToken     string
	InfluxdbOrg       string
	InfluxdbBucket    string
	InfluxdbBatchSize int

	PrometheusAddr string
}

// Service contains main service dependency.
type Service struct {
	Config        Config
	FederatorSvc  federator.Service
	PrometheusSvc prometheus.Service
}

// New service.
func New(config Config, federatorSvc federator.Service, prometheusSvc prometheus.Service) Service {
	return Service{
		Config:        config,
		FederatorSvc:  federatorSvc,
		PrometheusSvc: prometheusSvc,
	}
}

// Run main service.
func (s Service) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	log.Info("Start Cron scheduler")
	cronScheduler := cron.New(cron.WithSeconds())
	_, err := cronScheduler.AddFunc(s.Config.CronJobSchedule, s.TrafficBandwidthJobFunc)
	if err != nil {
		return fmt.Errorf("error adding TrafficBandwidthJobFunc function to Cron scheduler: %w", err)
	}
	_, err = cronScheduler.AddFunc(s.Config.CronJobSchedule, s.UpstreamServicesJobFunc)
	if err != nil {
		return fmt.Errorf("error adding UpstreamServicesJobFunc function to Cron scheduler: %w", err)
	}
	_, err = cronScheduler.AddFunc(s.Config.CronJobSchedule, s.DownstreamServicesJobFunc)
	if err != nil {
		return fmt.Errorf("error adding DownstreamServicesJobFunc function to Cron scheduler: %w", err)
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

			log.Info("Flush any pending federator backend writes")
			s.FederatorSvc.Flush()

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

// TrafficBandwidthJobFunc queries traffic bandwidth (planet-exporter) data from Prometheus and store
// them in federator backend.
func (s Service) TrafficBandwidthJobFunc() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.Config.CronJobTimeoutSecond)*time.Second)
	defer cancel()

	jobStartTime := s.getCronJobStartTime()
	log.Debugf("A job started: %v", jobStartTime)

	trafficPeers, err := s.PrometheusSvc.QueryPlanetExporterTrafficBandwidth(ctx, jobStartTime.Add(-15*time.Second), jobStartTime)
	if err != nil {
		log.Errorf("Error querying traffic peers from prometheus: %v", err)
	}

	for _, trafficPeer := range trafficPeers {
		_ = s.FederatorSvc.AddTrafficBandwidthData(ctx, federator.TrafficBandwidth{
			LocalHostgroup:  trafficPeer.LocalHostgroup,
			LocalAddress:    trafficPeer.LocalDomain,
			RemoteHostgroup: trafficPeer.RemoteHostgroup,
			RemoteDomain:    trafficPeer.RemoteDomain,
			BitsPerSecond:   trafficPeer.BandwidthBitsPerSecond,
			Direction:       trafficPeer.Direction,
		}, jobStartTime)
	}

	log.Infof("Traffic Bandwidth Job took: %v", s.getCronJobDuration(jobStartTime))
}

// UpstreamServicesJobFunc queries upstream services (planet-exporter) data from Prometheus and store
// them in federator backend.
func (s Service) UpstreamServicesJobFunc() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.Config.CronJobTimeoutSecond)*time.Second)
	defer cancel()

	jobStartTime := s.getCronJobStartTime()
	log.Debugf("A job started: %v", jobStartTime)

	upstreamServices, err := s.PrometheusSvc.QueryPlanetExporterUpstreamServices(ctx, jobStartTime.Add(-15*time.Second), jobStartTime)
	if err != nil {
		log.Errorf("Error querying upstream services from prometheus: %v", err)
	}

	for _, svc := range upstreamServices {
		_ = s.FederatorSvc.AddUpstreamService(ctx, federator.UpstreamService{
			LocalProcessName:  svc.LocalProcessName,
			LocalHostgroup:    svc.LocalHostgroup,
			LocalAddress:      svc.LocalAddress,
			UpstreamHostgroup: svc.RemoteHostgroup,
			UpstreamAddress:   svc.RemoteAddress,
			UpstreamPort:      svc.Port,
			Protocol:          svc.Protocol,
		}, jobStartTime)
	}

	log.Infof("Upstream Service Job took: %v", s.getCronJobDuration(jobStartTime))
}

// DownstreamServicesJobFunc queries downstream services (planet-exporter) data from Prometheus and store
// them in federator backend.
func (s Service) DownstreamServicesJobFunc() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.Config.CronJobTimeoutSecond)*time.Second)
	defer cancel()

	jobStartTime := s.getCronJobStartTime()
	log.Debugf("A job started: %v", jobStartTime)

	downstreamServices, err := s.PrometheusSvc.QueryPlanetExporterDownstreamServices(ctx, jobStartTime.Add(-15*time.Second), jobStartTime)
	if err != nil {
		log.Errorf("Error querying downstream services from prometheus: %v", err)
	}

	for _, svc := range downstreamServices {
		_ = s.FederatorSvc.AddDownstreamService(ctx, federator.DownstreamService{
			LocalProcessName:    svc.LocalProcessName,
			LocalHostgroup:      svc.LocalHostgroup,
			LocalAddress:        svc.LocalAddress,
			DownstreamHostgroup: svc.RemoteHostgroup,
			DownstreamAddress:   svc.RemoteAddress,
			LocalPort:           svc.Port,
			Protocol:            svc.Protocol,
		}, jobStartTime)
	}

	log.Infof("Downstream Service Job took: %v", s.getCronJobDuration(jobStartTime))
}
