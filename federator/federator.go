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

// TrafficBandwidth represents a pair of services that are involved in an ingress/egress traffic
// e.g. LocalHostgroup testapp transmit 10Mbps to RemoteHostgroup abc
type TrafficBandwidth struct {
	LocalHostgroup  string
	LocalAddress    string
	RemoteHostgroup string
	RemoteDomain    string
	BitsPerSecond   float64
	Direction       string
}

// UpstreamService represents a target upstream service dependency of a local service process
// e.g. LocalHostgroup testapp depends on UpstreamHostgroup abc, on abc's port 9000 via TCP protocol.
//      LocalHostgroup -> UpstreamHostgroup:UpstreamPort
//      testapp        -> abc:9000 (upstream)
type UpstreamService struct {
	LocalHostgroup    string
	LocalAddress      string
	LocalProcessName  string
	UpstreamPort      string
	UpstreamHostgroup string
	UpstreamAddress   string
	Protocol          string
}

// DownstreamService represents a target downstream service that depends on local service process
// e.g. LocalHostgroup testapp has a dependency DownstreamHostgroup abc, on testapp's port 80 via TCP protocol.
//      LocalHostgroup:LocalPort <- DownstreamHostgroup
//      testapp:80               <- abc (downstream)
type DownstreamService struct {
	LocalHostgroup      string
	LocalAddress        string
	LocalProcessName    string
	LocalPort           string
	DownstreamHostgroup string
	DownstreamAddress   string
	Protocol            string
}

// Backend interface for a time-series DB that is handling pre-processed planet-exporter data
// Planet Expoter <- Prometheus -> Planet Federator (pre-process) -> Time-series DB
type Backend interface {
	AddTrafficBandwidthData(context.Context, TrafficBandwidth, time.Time) error
	AddUpstreamService(context.Context, UpstreamService, time.Time) error
	AddDownstreamService(context.Context, DownstreamService, time.Time) error
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

// AddUpstreamService adds an upstream of a local service
func (s Service) AddUpstreamService(ctx context.Context, upstreamService UpstreamService, t time.Time) error {
	return s.backend.AddUpstreamService(ctx, upstreamService, t)
}

// AddDownstreamService adds a downstream of a local service
func (s Service) AddDownstreamService(ctx context.Context, downstreamService DownstreamService, t time.Time) error {
	return s.backend.AddDownstreamService(ctx, downstreamService, t)
}

// Flush any buffers related to backend
func (s Service) Flush() {
	s.backend.Flush()
}
