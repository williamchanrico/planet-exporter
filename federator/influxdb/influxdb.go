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

package influxdb

import (
	"context"
	"time"

	"planet-exporter/federator"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	influxdb2api "github.com/influxdata/influxdb-client-go/v2/api"
	log "github.com/sirupsen/logrus"
)

// Backend interface for a time-series DB handling pre-processed planet-exporter data
type Backend struct {
	client   influxdb2.Client
	writeAPI influxdb2api.WriteAPI
	org      string
	bucket   string
}

// New returns new influxdb federator backend
func New(c influxdb2.Client, org, bucket string) Backend {
	writeAPI := c.WriteAPI(org, bucket)

	errChan := writeAPI.Errors()
	go func() {
		for err := range errChan {
			log.Errorf("Async error received on influxdb writes API: %v", err)
		}
	}()

	return Backend{
		client:   c,
		writeAPI: writeAPI,
		org:      org,
		bucket:   bucket,
	}
}

const (
	// Measurements

	upstreamServiceMeasurement   = "upstream"
	downstreamServiceMeasurement = "downstream"

	ingressDirectionMeasurement = "ingress"
	egressDirectionMeasurement  = "egress"
	unknownDirectionMeasurement = "unknown"

	// Tags

	localServiceHostgroupTag   = "service"
	localServiceAddressTag     = "address"
	localServicePortTag        = "port"
	localServiceProcessNameTag = "process_name"

	remoteServiceHostgroupTag = "remote_service"
	remoteServiceAddressTag   = "remote_address"

	upstreamServiceHostgroupTag = "upstream_service"
	upstreamServiceAddressTag   = "upstream_address"
	upstreamServicePortTag      = "upstream_port"

	downstreamServiceHostgroupTag = "upstream_service"
	downstreamServiceAddressTag   = "upstream_address"

	protocolTag = "protocol"

	// Fields

	bandwidthBpsField      = "bandwidth_bps"
	serviceDependencyField = "service_dependency"
)

// AddTrafficBandwidthData adds a service's ingress bytes data point
// Example InfluxQL:
//   SELECT
//     SUM("bandwidth_bps")
//   FROM
//     "ingress"
//   WHERE
//     ("service" = '$service') AND $timeFilter
//   GROUP BY
//     time($__interval), "service", "remote_service", "remote_address"
//
func (b Backend) AddTrafficBandwidthData(ctx context.Context, trafficBandwidth federator.TrafficBandwidth, t time.Time) error {
	measurement := ""
	switch trafficBandwidth.Direction {
	case "ingress":
		measurement = ingressDirectionMeasurement
	case "egress":
		measurement = egressDirectionMeasurement
	default:
		measurement = unknownDirectionMeasurement
	}
	return b.addBytesMeasurement(ctx, measurement, trafficBandwidth, t)
}

func (b Backend) addBytesMeasurement(ctx context.Context, measurement string, trafficBandwidth federator.TrafficBandwidth, t time.Time) error {
	dataPoint := influxdb2.NewPointWithMeasurement(measurement).
		AddTag(localServiceHostgroupTag, trafficBandwidth.LocalHostgroup).
		AddTag(localServiceAddressTag, trafficBandwidth.LocalAddress).
		AddTag(remoteServiceHostgroupTag, trafficBandwidth.RemoteHostgroup).
		AddTag(remoteServiceAddressTag, trafficBandwidth.RemoteDomain).
		AddField(bandwidthBpsField, trafficBandwidth.BitsPerSecond).
		SetTime(time.Now())
	b.writeAPI.WritePoint(dataPoint)

	return nil
}

// AddUpstreamService adds an upstream service dependency of a service
func (b Backend) AddUpstreamService(ctx context.Context, upstreamService federator.UpstreamService, t time.Time) error {
	dataPoint := influxdb2.NewPointWithMeasurement(upstreamServiceMeasurement).
		AddTag(localServiceHostgroupTag, upstreamService.LocalHostgroup).
		AddTag(localServiceAddressTag, upstreamService.LocalAddress).
		AddTag(upstreamServiceHostgroupTag, upstreamService.UpstreamHostgroup).
		AddTag(upstreamServiceAddressTag, upstreamService.UpstreamAddress).
		AddTag(upstreamServicePortTag, upstreamService.UpstreamPort).
		AddTag(localServiceProcessNameTag, upstreamService.LocalProcessName).
		AddTag(protocolTag, upstreamService.Protocol).
		AddField(serviceDependencyField, 1).
		SetTime(t)
	b.writeAPI.WritePoint(dataPoint)

	return nil
}

// AddDownstreamService adds a downstream service dependency of a service
func (b Backend) AddDownstreamService(ctx context.Context, downstreamService federator.DownstreamService, t time.Time) error {
	dataPoint := influxdb2.NewPointWithMeasurement(upstreamServiceMeasurement).
		AddTag(localServiceHostgroupTag, downstreamService.LocalHostgroup).
		AddTag(localServiceAddressTag, downstreamService.LocalAddress).
		AddTag(localServicePortTag, downstreamService.LocalPort).
		AddTag(localServiceProcessNameTag, downstreamService.LocalProcessName).
		AddTag(downstreamServiceHostgroupTag, downstreamService.DownstreamHostgroup).
		AddTag(downstreamServiceAddressTag, downstreamService.DownstreamAddress).
		AddTag(protocolTag, downstreamService.Protocol).
		AddField(serviceDependencyField, 1).
		SetTime(t)
	b.writeAPI.WritePoint(dataPoint)

	return nil
}

// Flush all influxdb writes
func (b Backend) Flush() {
	b.writeAPI.Flush()
}
