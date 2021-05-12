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

// PlanetExporterTrafficBandwidth represents a single traffic between local and remote hostgroup
type PlanetExporterTrafficBandwidth struct {
	LocalHostgroup         string  `json:"local_hostgroup"` // e.g. hostgroup
	RemoteHostgroup        string  `json:"remote_hostgroup"`
	LocalDomain            string  `json:"local_domain"` // e.g. consul domain
	RemoteDomain           string  `json:"remote_domain"`
	BandwidthBitsPerSecond float64 `json:"bandwidth_bits_per_second"`
	Direction              string  `json:"direction"`
}

// QueryPlanetExporterTrafficBandwidth returns list traffic bandwidth data
func (s Service) QueryPlanetExporterTrafficBandwidth(ctx context.Context, startTime time.Time, endTime time.Time) ([]PlanetExporterTrafficBandwidth, error) {
	// query data as bits per second and only those higher than 1Kbps to reduce noise
	qr := fmt.Sprintf(`
			sum (
				sum (
					irate (planet_traffic_bytes_total{local_hostgroup!="", remote_ip!~"%v", remote_domain!~"%v", remote_hostgroup!=""}[30s])
				) by (direction, local_hostgroup, local_domain, remote_hostgroup, remote_domain, instance) * 8
			)
			by (direction, local_hostgroup, local_domain, remote_hostgroup, remote_domain) > 1000`,
		regexExcludedAddresses, regexExcludedAddresses)
	qrTrafficPeers, err := s.queryRange(ctx, qr, startTime, endTime)
	if err != nil {
		return nil, err
	}

	trafficBandwidthData := []PlanetExporterTrafficBandwidth{}
	for _, v := range qrTrafficPeers.(model.Matrix) {
		localHostgroup, ok := v.Metric["local_hostgroup"]
		if !ok {
			log.Warnf("Found empty local_hostgroup: %v", v.Metric.String())
			continue
		}
		localDomain := v.Metric["local_domain"]
		remoteHostgroup := v.Metric["remote_hostgroup"]
		remoteDomain := v.Metric["remote_domain"]
		direction := v.Metric["direction"]

		bw := s.getMaxValueFromSamplePairs(v.Values)

		trafficBandwidthData = append(trafficBandwidthData, PlanetExporterTrafficBandwidth{
			Direction:              string(direction),
			LocalHostgroup:         string(localHostgroup),
			RemoteHostgroup:        string(remoteHostgroup),
			LocalDomain:            string(localDomain),
			RemoteDomain:           string(remoteDomain),
			BandwidthBitsPerSecond: bw,
		})
	}

	return trafficBandwidthData, nil
}

// PlanetExporterUpstreamService represents an upstream service dependency of a local service
type PlanetExporterUpstreamService struct {
	LocalHostgroup    string `json:"local_hostgroup"`
	LocalAddress      string `json:"local_address"`
	LocalProcessName  string `json:"local_process_name"`
	UpstreamPort      string `json:"upstream_port"`
	UpstreamHostgroup string `json:"upstream_hostgroup"`
	UpstreamAddress   string `json:"upstream_address"`
	Protocol          string `json:"protocol"`
}

// QueryPlanetExporterUpstreamServices returns all upstream service dependencies
func (s Service) QueryPlanetExporterUpstreamServices(ctx context.Context, startTime time.Time, endTime time.Time) ([]PlanetExporterUpstreamService, error) {
	qr := fmt.Sprintf(`
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
	qrUpstreamServices, err := s.queryRange(ctx, qr, startTime, endTime)
	if err != nil {
		return nil, err
	}

	upstreamServices := []PlanetExporterUpstreamService{}
	for _, v := range qrUpstreamServices.(model.Matrix) {
		localHostgroup, ok := v.Metric["local_hostgroup"]
		if !ok {
			log.Warnf("Found empty local_hostgroup: %v", v.Metric.String())
			continue
		}
		localAddress := v.Metric["local_address"]
		localProcessName := v.Metric["process_name"]
		upstreamPort := v.Metric["port"]
		upstreamHostgroup := v.Metric["remote_hostgroup"]
		upstreamAddress := v.Metric["remote_address"]

		upstreamServices = append(upstreamServices, PlanetExporterUpstreamService{
			LocalHostgroup:    string(localHostgroup),
			LocalAddress:      string(localAddress),
			LocalProcessName:  string(localProcessName),
			UpstreamPort:      string(upstreamPort),
			UpstreamHostgroup: string(upstreamHostgroup),
			UpstreamAddress:   string(upstreamAddress),
		})
	}

	return upstreamServices, nil
}

// PlanetExporterDownstreamService represents an downstream service dependency of a local service
type PlanetExporterDownstreamService struct {
	LocalHostgroup      string `json:"local_hostgroup"`
	LocalAddress        string `json:"local_address"`
	LocalProcessName    string `json:"local_process_name"`
	LocalPort           string `json:"local_port"`
	DownstreamHostgroup string `json:"downstream_hostgroup"`
	DownstreamAddress   string `json:"downstream_address"`
	Protocol            string `json:"protocol"`
}

// QueryPlanetExporterDownstreamServices returns all downstream service dependencies
func (s Service) QueryPlanetExporterDownstreamServices(ctx context.Context, startTime time.Time, endTime time.Time) ([]PlanetExporterDownstreamService, error) {
	qr := fmt.Sprintf(`
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
	qrDownstreamServices, err := s.queryRange(ctx, qr, startTime, endTime)
	if err != nil {
		return nil, err
	}

	downstreamServices := []PlanetExporterDownstreamService{}
	for _, v := range qrDownstreamServices.(model.Matrix) {
		localHostgroup, ok := v.Metric["local_hostgroup"]
		if !ok {
			log.Warnf("Found empty local_hostgroup: %v", v.Metric.String())
			continue
		}
		localAddress := v.Metric["local_address"]
		localProcessName := v.Metric["process_name"]
		localPort := v.Metric["port"]
		downstreamHostgroup := v.Metric["remote_hostgroup"]
		downstreamAddress := v.Metric["remote_address"]

		downstreamServices = append(downstreamServices, PlanetExporterDownstreamService{
			LocalHostgroup:      string(localHostgroup),
			LocalAddress:        string(localAddress),
			LocalProcessName:    string(localProcessName),
			LocalPort:           string(localPort),
			DownstreamHostgroup: string(downstreamHostgroup),
			DownstreamAddress:   string(downstreamAddress),
		})
	}

	return downstreamServices, nil
}
