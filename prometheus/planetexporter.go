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

package prometheus

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/common/model"
	log "github.com/sirupsen/logrus"
)

const (
	regexExcludedPorts     = "(22|53|111|8301|8300|8500|3025|3022|51666|9100|19100|5666|25|8600|11910|11560)"
	regexExcludedAddresses = "(100.([6-9]|1[0-2]).*|52.*|192.168.*|.*prometheus.*|203.*|163.18.*|130.211.*|f.*|169.254.*|111.*)"
)

// PlanetExporterTrafficBandwidth represents a single traffic between local and remote hostgroup.
type PlanetExporterTrafficBandwidth struct {
	LocalHostgroup         string  `json:"local_hostgroup"` // e.g. hostgroup
	RemoteHostgroup        string  `json:"remote_hostgroup"`
	LocalDomain            string  `json:"local_domain"` // e.g. consul domain
	RemoteDomain           string  `json:"remote_domain"`
	BandwidthBitsPerSecond float64 `json:"bandwidth_bits_per_second"`
	Direction              string  `json:"direction"`
}

// QueryPlanetExporterTrafficBandwidth returns list traffic bandwidth data.
func (s Service) QueryPlanetExporterTrafficBandwidth(ctx context.Context, startTime time.Time, endTime time.Time) ([]PlanetExporterTrafficBandwidth, error) {
	// query data as bits per second and only those higher than 1Kbps to reduce noise
	// include remote services (hostgroup and domain) in the result
	qrWithRemoteServices := fmt.Sprintf(`
			sum (
				sum (
					irate (planet_traffic_bytes_total{local_hostgroup!="", remote_ip!~"%v", remote_domain!~"%v", remote_hostgroup!=""}[30s])
				) by (direction, local_hostgroup, local_domain, remote_hostgroup, remote_domain, instance) * 8
			)
			by (direction, local_hostgroup, local_domain, remote_hostgroup, remote_domain) > 1000`,
		regexExcludedAddresses, regexExcludedAddresses)
	withRemoteServices, err := s.queryPlanetExporterTrafficBandwidth(ctx, qrWithRemoteServices, startTime, endTime)
	if err != nil {
		return nil, err
	}

	trafficBandwidthData := []PlanetExporterTrafficBandwidth{}
	trafficBandwidthData = append(trafficBandwidthData, withRemoteServices...)

	return trafficBandwidthData, nil
}

func (s Service) queryPlanetExporterTrafficBandwidth(ctx context.Context, query string, startTime time.Time, endTime time.Time) ([]PlanetExporterTrafficBandwidth, error) {
	qrTrafficPeers, err := s.queryRange(ctx, query, startTime, endTime)
	if err != nil {
		return nil, err
	}

	trafficBandwidthData := []PlanetExporterTrafficBandwidth{}
	for _, matrix := range qrTrafficPeers.(model.Matrix) {
		localHostgroup, ok := matrix.Metric["local_hostgroup"]
		if !ok {
			log.Warnf("Found empty local_hostgroup: %v", matrix.Metric.String())

			continue
		}
		localDomain := matrix.Metric["local_domain"]
		remoteHostgroup := matrix.Metric["remote_hostgroup"]
		remoteDomain := matrix.Metric["remote_domain"]
		direction := matrix.Metric["direction"]

		bandwidthBitsPerSecond := s.getMaxValueFromSamplePairs(matrix.Values)

		trafficBandwidthData = append(trafficBandwidthData, PlanetExporterTrafficBandwidth{
			Direction:              string(direction),
			LocalHostgroup:         string(localHostgroup),
			RemoteHostgroup:        string(remoteHostgroup),
			LocalDomain:            string(localDomain),
			RemoteDomain:           string(remoteDomain),
			BandwidthBitsPerSecond: bandwidthBitsPerSecond,
		})
	}

	return trafficBandwidthData, nil
}

// PlanetExporterDependencyService represents an upstream/downstream service dependency of a local service.
type PlanetExporterDependencyService struct {
	LocalHostgroup  string
	LocalAddress    string
	RemoteHostgroup string
	RemoteAddress   string

	// LocalProcessName represents the process that interacts with the upstream/downstream dependency.
	LocalProcessName string

	// Port represents the port that is depended upon.
	// This would be a remote port for an upstream dependency and a local port for a downstream dependency.
	//
	// Example: Server --> (remote port) Upstream || Downstream --> (local port) Server
	Port string

	Protocol string
}

// QueryPlanetExporterUpstreamServices returns all upstream service dependencies.
func (s Service) QueryPlanetExporterUpstreamServices(ctx context.Context, startTime time.Time, endTime time.Time) ([]PlanetExporterDependencyService, error) {
	query := fmt.Sprintf(`
			max(
				max_over_time(
					planet_upstream{
						local_hostgroup!="",
						port!~"%v",
						remote_address!~"%v",
						remote_address!="localhost",
						process_name!="",
						remote_address!~"\\d.*"
					}[15s]
				)
			) by (local_hostgroup, local_address, remote_address, remote_hostgroup, port, process_name, protocol)`,
		regexExcludedPorts, regexExcludedAddresses)

	dependencyServices, err := s.queryPlanetExporterDependencyServices(ctx, query, startTime, endTime)
	if err != nil {
		return nil, err
	}

	return dependencyServices, nil
}

// QueryPlanetExporterDownstreamServices returns all downstream service dependencies.
func (s Service) QueryPlanetExporterDownstreamServices(ctx context.Context, startTime time.Time, endTime time.Time) ([]PlanetExporterDependencyService, error) {
	query := fmt.Sprintf(`
			max(
				max_over_time(
					planet_downstream{
						local_hostgroup!="",
						port!~"%v",
						remote_address!~"%v",
						remote_address!="localhost",
						process_name!="",
						remote_address!~"\\d.*"
					}[15s]
				)
			) by (local_hostgroup, local_address, remote_address, remote_hostgroup, port, process_name, protocol)`,
		regexExcludedPorts, regexExcludedAddresses)

	downstreamServices, err := s.queryPlanetExporterDependencyServices(ctx, query, startTime, endTime)
	if err != nil {
		return nil, err
	}

	return downstreamServices, nil
}

func (s Service) queryPlanetExporterDependencyServices(ctx context.Context, query string, startTime, endTime time.Time) ([]PlanetExporterDependencyService, error) {
	resultDependencyServices, err := s.queryRange(ctx, query, startTime, endTime)
	if err != nil {
		return nil, err
	}

	dependencyServices := []PlanetExporterDependencyService{}
	for _, matrix := range resultDependencyServices.(model.Matrix) {
		localHostgroup, ok := matrix.Metric["local_hostgroup"]
		if !ok {
			log.Warnf("Found empty local_hostgroup: %v", matrix.Metric.String())

			continue
		}
		localAddress := matrix.Metric["local_address"]
		localProcessName := matrix.Metric["process_name"]
		remotePort := matrix.Metric["port"]
		remoteHostgroup := matrix.Metric["remote_hostgroup"]
		remoteAddress := matrix.Metric["remote_address"]
		protocol := matrix.Metric["protocol"]

		dependencyServices = append(dependencyServices, PlanetExporterDependencyService{
			LocalHostgroup:   string(localHostgroup),
			LocalAddress:     string(localAddress),
			LocalProcessName: string(localProcessName),
			Port:             string(remotePort),
			RemoteHostgroup:  string(remoteHostgroup),
			RemoteAddress:    string(remoteAddress),
			Protocol:         string(protocol),
		})
	}

	return dependencyServices, nil
}
