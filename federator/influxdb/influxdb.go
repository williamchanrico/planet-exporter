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
	return Backend{
		client:   c,
		writeAPI: c.WriteAPI(org, bucket),
		org:      org,
		bucket:   bucket,
	}
}

const (
	ingressDirectionMeasurement = "ingress"
	egressDirectionMeasurement  = "egress"
	unknownDirectionMeasurement = "unknown"

	localServiceHostgroupTag = "service"
	localServiceAddressTag   = "address"

	remoteServiceHostgroupTag = "remote_service"
	remoteServiceAddressTag   = "remote_address"

	bandwidthBpsField = "bandwidth_bps"
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

// Flush all influxdb writes
func (b Backend) Flush() {
	b.writeAPI.Flush()
}
