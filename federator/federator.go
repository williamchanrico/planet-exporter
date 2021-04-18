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

package federator

import (
	"context"
	"time"
)

// Federator package handles storing pre-processed planet-exporter data from Prometheus to
// Influxdb and/or other time-series databases.

// TrafficBandwidth represents services involved in an ingress/egress traffic
type TrafficBandwidth struct {
	LocalHostgroup  string
	LocalAddress    string
	RemoteHostgroup string
	RemoteDomain    string
	BitsPerSecond   float64
	Direction       string
}

// Backend interface for a time-series DB handling pre-processed planet-exporter data
type Backend interface {
	AddTrafficBandwidthData(context.Context, TrafficBandwidth, time.Time) error
	Flush()
}

// Service represents a federator service
type Service struct {
	backend Backend
}

// New returns new federator service
func New(b Backend) Service {
	return Service{
		backend: b,
	}
}

// AddTrafficBandwidthData adds an ingress bytes data point
func (s Service) AddTrafficBandwidthData(ctx context.Context, trafficBandwidth TrafficBandwidth, t time.Time) error {
	return s.backend.AddTrafficBandwidthData(ctx, trafficBandwidth, t)
}

// Flush any buffers related to backend
func (s Service) Flush() {
	s.backend.Flush()
}
