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

package query

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"

	influxdb1 "github.com/influxdata/influxdb1-client/v2"
	log "github.com/sirupsen/logrus"
)

// Client for InfluxDB.
type Client struct {
	client   influxdb1.Client
	database string
}

// New client for querying InfluxDB client compatible with planet-federator (currently using v1).
func New(client influxdb1.Client, database string) *Client {
	return &Client{
		client:   client,
		database: database,
	}
}

// TrafficBandwidth represents federator traffic bandwidth data.
type TrafficBandwidth struct {
	TrafficDirection          string `json:"traffic_direction"`
	LocalHostgroup            string `json:"local_hostgroup"`
	LocalHostgroupAddress     string `json:"local_hostgroup_address"`
	RemoteHostgroup           string `json:"remote_hostgroup"`
	RemoteHostgroupAddress    string `json:"remote_hostgroup_address"`
	TrafficBandwidthBitsMin1h int64  `json:"traffic_bandwidth_bits_min_1h"`
	TrafficBandwidthBitsMax1h int64  `json:"traffic_bandwidth_bits_max_1h"`
	TrafficBandwidthBitsAvg1h int64  `json:"traffic_bandwidth_bits_avg_1h"`
}

// QueryFederatorTraffic returns ingress & egress federator traffic data from InfluxDB.
func (c *Client) QueryFederatorTraffic(ctx context.Context) ([]TrafficBandwidth, error) {
	trafficData := []TrafficBandwidth{}

	queryParamMatrix := [][]string{
		{"ingress", "1h"},
		{"egress", "1h"},
	}
	for _, v := range queryParamMatrix {
		queryParamDirection := v[0]
		queryParamTimeRange := v[1]
		log.Debugf("queryParamMatrix direction=%v, timerange=%v", queryParamDirection, queryParamTimeRange)

		q := `
			SELECT
				MIN("bandwidth_bps"), MAX("bandwidth_bps"), MEAN("bandwidth_bps")
			FROM
				%v
			WHERE
				("service" != '') AND time > now() - %v
			GROUP BY
				service, address, remote_service, remote_address
		`
		renderedQuery := fmt.Sprintf(q, queryParamDirection, queryParamTimeRange)

		query := influxdb1.NewQuery(renderedQuery, c.database, "")
		results, err := c.queryFederatorTrafficData(ctx, query)
		if err != nil {
			return []TrafficBandwidth{}, errors.Wrapf(err, "failed to query %v traffic data for time range %v", queryParamDirection, queryParamTimeRange)
		}

		trafficData = append(trafficData, results...)
	}

	return trafficData, nil
}

// queryFederatorTrafficData executes the traffic query on InfluxDB and stores the result.
func (c *Client) queryFederatorTrafficData(ctx context.Context, query influxdb1.Query) ([]TrafficBandwidth, error) {
	resp, err := c.client.Query(query)
	if err != nil {
		return []TrafficBandwidth{}, errors.Wrap(err, "failed to query QueryFederatorTraffic")
	}
	if resp.Error() != nil {
		return []TrafficBandwidth{}, errors.Wrap(resp.Error(), "received invalid response")
	}
	if len(resp.Results) == 0 || len(resp.Results[0].Series) == 0 {
		return []TrafficBandwidth{}, errors.New("received empty data")
	}

	trafficData := []TrafficBandwidth{}

	for _, series := range resp.Results[0].Series {
		for _, row := range series.Values {
			TrafficBandwidthBitsMin1h, err := transformJSONNumberToInteger(row[1])
			if err != nil {
				log.Warnf("error transformJSONNumberToInteger for %v: %v", row[1], err)
				continue
			}
			TrafficBandwidthBitsMax1h, err := transformJSONNumberToInteger(row[2])
			if err != nil {
				log.Warnf("error transformJSONNumberToInteger for %v: %v", row[2], err)
				continue
			}
			TrafficBandwidthBitsAvg1h, err := transformJSONNumberToInteger(row[3])
			if err != nil {
				log.Warnf("error transformJSONNumberToInteger for %v: %v", row[3], err)
				continue
			}

			traffic := TrafficBandwidth{
				TrafficDirection:          series.Name,
				LocalHostgroup:            series.Tags["service"],
				LocalHostgroupAddress:     series.Tags["address"],
				RemoteHostgroup:           series.Tags["remote_service"],
				RemoteHostgroupAddress:    series.Tags["remote_address"],
				TrafficBandwidthBitsMin1h: TrafficBandwidthBitsMin1h,
				TrafficBandwidthBitsMax1h: TrafficBandwidthBitsMax1h,
				TrafficBandwidthBitsAvg1h: TrafficBandwidthBitsAvg1h,
			}
			trafficData = append(trafficData, traffic)

			// log.Debugf("queryFederatorTrafficData new entry: %+v", traffic)
		}
	}
	return trafficData, nil
}

func transformJSONNumberToInteger(i interface{}) (int64, error) {
	jsonNumber, ok := i.(json.Number)
	if !ok {
		return -1, fmt.Errorf("error on type assertion")
	}

	// Throw away decimals
	integerString := strings.Split(jsonNumber.String(), ".")[0]
	result, err := strconv.ParseInt(integerString, 10, 64)
	if err != nil {
		return -1, errors.Wrapf(err, "error converting %v to int", integerString)
	}

	return result, nil

}

// Dependency represents a dependency data.
type Dependency struct {
	// Direction determines whether it's an upstream/downstream dependency.
	Direction                 string `json:"direction"`
	Protocol                  string `json:"protocol"`
	LocalHostgroupProcessName string `json:"local_hostgroup_process_name"`

	LocalHostgroup        string `json:"local_hostgroup"`
	LocalHostgroupAddress string `json:"local_hostgroup_address"`

	// LocalHostgroupPort is only relevant for dependencyDirection=downstream
	// This signifies which local port that the downstream connected to.
	LocalHostgroupAddressPort string `json:"local_hostgroup_address_port"`

	RemoteHostgroup        string `json:"remote_hostgroup"`
	RemoteHostgroupAddress string `json:"remote_hostgroup_address"`

	// RemoteHostgroupPort is only relevant for dependencyDirection=upstream
	// This signifies the upstream port.
	RemoteHostgroupAddressPort string `json:"remote_hostgroup_address_port"`
}

// QueryFederatorDependencyLast7d returns last 7d federator upstream & downstream data.
func (c *Client) QueryFederatorDependencyLast7d(ctx context.Context) ([]Dependency, error) {
	dependencyData := []Dependency{}

	qUpstream := `
		SELECT
			COUNT(*)
		FROM
			upstream
		WHERE
			("service" != '') AND time > now() - 7d
		GROUP BY
			service, upstream_service, upstream_address, process_name, upstream_port, protocol, time(1000d)
	`

	query := influxdb1.NewQuery(qUpstream, c.database, "")
	upstreamData, err := c.queryFederatorDependencyData(ctx, query)
	if err != nil {
		return []Dependency{}, errors.Wrap(err, "failed to query ingress traffic data")
	}

	qDownstream := `
		SELECT
			COUNT(*)
		FROM
			downstream
		WHERE
			("service" != '') AND time > now() - 7d
		GROUP BY
			service, downstream_service, downstream_address, process_name, port, protocol, time(1000d)
	`

	query = influxdb1.NewQuery(qDownstream, c.database, "")
	downstreamData, err := c.queryFederatorDependencyData(ctx, query)
	if err != nil {
		return []Dependency{}, errors.Wrap(err, "failed to query egress traffic data")
	}

	dependencyData = append(dependencyData, upstreamData...)
	dependencyData = append(dependencyData, downstreamData...)

	return dependencyData, nil
}

// queryFederatorDependencyData executes the dependency data query on InfluxDB and stores the result.
func (c *Client) queryFederatorDependencyData(ctx context.Context, query influxdb1.Query) ([]Dependency, error) {
	resp, err := c.client.Query(query)
	if err != nil {
		return []Dependency{}, errors.Wrap(err, "failed to query QueryFederatorTraffic")
	}
	if resp.Error() != nil {
		return []Dependency{}, errors.Wrap(resp.Error(), "received invalid response")
	}
	if len(resp.Results) == 0 || len(resp.Results[0].Series) == 0 {
		return []Dependency{}, errors.New("received empty data")
	}

	dependencyData := []Dependency{}

	for _, series := range resp.Results[0].Series {
		remoteHostgroup := series.Tags["downstream_service"]
		if series.Name == "upstream" {
			remoteHostgroup = series.Tags["upstream_service"]
		}

		remoteAddress := series.Tags["downstream_address"]
		if series.Name == "upstream" {
			remoteAddress = series.Tags["upstream_address"]
		}

		dependency := Dependency{
			Direction:                  series.Name,
			Protocol:                   series.Tags["protocol"],
			LocalHostgroupProcessName:  series.Tags["process_name"],
			LocalHostgroup:             series.Tags["service"],
			LocalHostgroupAddress:      series.Tags["address"],
			LocalHostgroupAddressPort:  series.Tags["port"],
			RemoteHostgroup:            remoteHostgroup,
			RemoteHostgroupAddress:     remoteAddress,
			RemoteHostgroupAddressPort: series.Tags["upstream_port"],
		}
		dependencyData = append(dependencyData, dependency)
	}
	return dependencyData, nil
}
