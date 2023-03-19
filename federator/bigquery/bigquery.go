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

package bigquery

import (
	"context"
	"fmt"
	"time"

	"planet-exporter/federator"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"
)

// Backend interface for a time-series DB handling pre-processed planet-exporter data.
type Backend struct {
	client *bigquery.Client

	trafficTable    *bigquery.Table
	dependencyTable *bigquery.Table
}

// TableMetadata represents a BigQuery Table Metadata.
type TableMetadata struct {
	DatasetID string
	TableID   string
}

// New returns new bigquery federator backend.
func New(bqClient *bigquery.Client, trafficTableMeta, dependencyTableMeta TableMetadata) Backend {
	trafficTable := bqClient.Dataset(trafficTableMeta.DatasetID).Table(trafficTableMeta.TableID)
	dependencyTable := bqClient.Dataset(dependencyTableMeta.DatasetID).Table(dependencyTableMeta.TableID)

	return Backend{
		client:          bqClient,
		trafficTable:    trafficTable,
		dependencyTable: dependencyTable,
	}
}

const (
	// Measurements.

	upstreamDependencyDirection   = "upstream"
	downstreamDependencyDirection = "downstream"

	ingressTrafficDirection = "ingress"
	egressTrafficDirection  = "egress"
	unknownTrafficDirection = "unknown"
)

// Schema - traffic
// [
//     {
//         "name": "inventory_date",
//         "type": "TIMESTAMP",
//         "mode": "REQUIRED"
//     },
//     {
//         "name": "traffic_direction",
//         "type": "STRING",
//         "mode": "REQUIRED",
//         "description": "The direction of the traffic. One of egress/ingress."
//     },
//     {
//         "name": "local_hostgroup",
//         "type": "STRING",
//         "mode": "REQUIRED",
//         "description": "The hostgroup handling the traffic."
//     },
//     {
//         "name": "local_hostgroup_address",
//         "type": "STRING",
//         "mode": "NULLABLE",
//         "description": "The address or URL of the local hostgroup. Usually a Consul domain."
//     },
//     {
//         "name": "remote_hostgroup",
//         "type": "STRING",
//         "mode": "REQUIRED",
//         "description": "The hostgroup that is sending/receiving traffic, depending on traffic direction."
//     },
//     {
//         "name": "remote_hostgroup_address",
//         "type": "STRING",
//         "mode": "NULLABLE",
//         "description": "The address or URL of the remote hostgroup. Usually a Consul domain."
//     },
//     {
//         "name": "traffic_bandwidth_bits",
//         "type": "INTEGER",
//         "mode": "REQUIRED",
//         "description": "The traffic bandwidth consumed in bit per second."
//     }
// ]

// TrafficTableSchema represents the schema for traffic table.
type TrafficTableSchema struct {
	InventoryDate          civil.DateTime      `json:"inventory_date"`
	TrafficDirection       string              `json:"traffic_direction"`
	LocalHostgroup         string              `json:"local_hostgroup"`
	LocalHostgroupAddress  bigquery.NullString `json:"local_hostgroup_address"`
	RemoteHostgroup        string              `json:"remote_hostgroup"`
	RemoteHostgroupAddress bigquery.NullString `json:"remote_hostgroup_address"`
	TrafficBandwidthBits   float64             `json:"traffic_bandwidth_bits"`
}

// AddTrafficBandwidthData adds a service's ingress bytes data point
func (b Backend) AddTrafficBandwidthData(ctx context.Context, trafficBandwidth federator.TrafficBandwidth, timeOfDataPoint time.Time) error {
	var direction string
	switch trafficBandwidth.Direction {
	case "ingress":
		direction = ingressTrafficDirection
	case "egress":
		direction = egressTrafficDirection
	default:
		direction = unknownTrafficDirection
	}

	return b.insertTraffic(ctx, direction, trafficBandwidth, timeOfDataPoint)
}

func (b Backend) insertTraffic(ctx context.Context, direction string, traffic federator.TrafficBandwidth, timeOfDataPoint time.Time) error { // nolint:unparam
	data := TrafficTableSchema{
		InventoryDate:          civil.DateTimeOf(timeOfDataPoint),
		TrafficDirection:       direction,
		LocalHostgroup:         traffic.LocalHostgroup,
		LocalHostgroupAddress:  bigquery.NullString{StringVal: traffic.LocalAddress},
		RemoteHostgroup:        traffic.RemoteHostgroup,
		RemoteHostgroupAddress: bigquery.NullString{StringVal: traffic.RemoteDomain},
		TrafficBandwidthBits:   traffic.BitsPerSecond,
	}

	inserter := b.trafficTable.Inserter()
	err := inserter.Put(ctx, data)
	if err != nil {
		if multiErr, ok := err.(bigquery.PutMultiError); ok {
			for _, putErr := range multiErr {
				// Return first one as a hint for the cause of error.
				return fmt.Errorf("failed to insert multiple rows to the traffic table, sample row %d, with err: %v", putErr.RowIndex, putErr.Error())
			}
		}
		return err
	}

	return nil
}

// Schema - dependency
// [
//     {
//         "name": "inventory_date",
//         "type": "TIMESTAMP",
//         "mode": "REQUIRED"
//     },
//     {
//         "name": "dependency_direction",
//         "type": "STRING",
//         "mode": "REQUIRED",
//         "description": "The relationship direction of the dependency, one of upstream/downstream."
//     },
//     {
//         "name": "protocol",
//         "type": "STRING",
//         "mode": "REQUIRED",
//         "description": "The L4 protocol of the dependency."
//     },
//     {
//         "name": "local_hostgroup_process_name",
//         "type": "STRING",
//         "mode": "NULLABLE",
//         "description": "The local process name that sends/receives the dependency traffic. May be null."
//     },
//     {
//         "name": "local_hostgroup",
//         "type": "STRING",
//         "mode": "REQUIRED",
//         "description": "The hostgroup handling the traffic."
//     },
//     {
//         "name": "local_hostgroup_address",
//         "type": "STRING",
//         "mode": "NULLABLE",
//         "description": "The address or URL of the local hostgroup. Usually a Consul domain."
//     },
//     {
//         "name": "local_hostgroup_address_port",
//         "type": "STRING",
//         "mode": "NULLABLE",
//         "description": "The local port that receives downstream traffic. May be null for an upstream dependency data."
//     },
//     {
//         "name": "remote_hostgroup",
//         "type": "STRING",
//         "mode": "REQUIRED",
//         "description": "The hostgroup that is sending/receiving traffic, depending on traffic direction."
//     },
//     {
//         "name": "remote_hostgroup_address",
//         "type": "STRING",
//         "mode": "NULLABLE",
//         "description": "The address or URL of the remote hostgroup. Usually a Consul domain."
//     },
//     {
//         "name": "remote_hostgroup_address_port",
//         "type": "STRING",
//         "mode": "NULLABLE",
//         "description": "The upstream port. May be null for a downstream data."
//     }
// ]

// DependencyTableSchema represents the schema for dependency table.
type DependencyTableSchema struct {
	InventoryDate civil.DateTime `json:"inventory_date"`

	// DependencyDirection determines whether it's an upstream/downstream dependency.
	DependencyDirection       string              `json:"dependency_direction"`
	Protocol                  string              `json:"protocol"`
	LocalHostgroupProcessName bigquery.NullString `json:"local_hostgroup_process_name"`

	LocalHostgroup        string              `json:"local_hostgroup"`
	LocalHostgroupAddress bigquery.NullString `json:"local_hostgroup_address"`

	// LocalHostgroupPort is only relevant for dependencyDirection=downstream
	// This signifies which local port that the downstream connected to.
	LocalHostgroupAddressPort bigquery.NullString `json:"local_hostgroup_address_port"`

	RemoteHostgroup        string              `json:"remote_hostgroup"`
	RemoteHostgroupAddress bigquery.NullString `json:"remote_hostgroup_address"`

	// RemoteHostgroupPort is only relevant for dependencyDirection=upstream
	// This signifies the upstream port.
	RemoteHostgroupAddressPort bigquery.NullString `json:"remote_hostgroup_address_port"`
}

// AddUpstreamService add an upstream service dependency of a service.
func (b Backend) AddUpstreamService(ctx context.Context, upstreamService federator.UpstreamService, timeOfDataPoint time.Time) error {
	localProcessName := bigquery.NullString{}
	if upstreamService.LocalProcessName != "" {
		localProcessName.StringVal = upstreamService.LocalAddress
	}
	localAddress := bigquery.NullString{}
	if upstreamService.LocalAddress != "" {
		localAddress.StringVal = upstreamService.LocalAddress
	}
	remoteAddress := bigquery.NullString{}
	if upstreamService.UpstreamAddress != "" {
		remoteAddress.StringVal = upstreamService.UpstreamAddress
	}
	remotePort := bigquery.NullString{}
	if upstreamService.UpstreamPort != "" {
		remotePort.StringVal = upstreamService.UpstreamPort
	}

	data := DependencyTableSchema{
		InventoryDate: civil.DateTimeOf(timeOfDataPoint),

		DependencyDirection:       upstreamDependencyDirection,
		Protocol:                  upstreamService.Protocol,
		LocalHostgroupProcessName: localProcessName,

		// This is null for an upstream dependency data
		LocalHostgroupAddressPort: bigquery.NullString{},

		LocalHostgroup:        upstreamService.LocalHostgroup,
		LocalHostgroupAddress: localAddress,

		RemoteHostgroup:        upstreamService.UpstreamHostgroup,
		RemoteHostgroupAddress: remoteAddress,

		RemoteHostgroupAddressPort: remotePort,
	}

	inserter := b.dependencyTable.Inserter()
	err := inserter.Put(ctx, data)
	if err != nil {
		if multiErr, ok := err.(bigquery.PutMultiError); ok {
			for _, putErr := range multiErr {
				// Return first one as a hint for the cause of error.
				return fmt.Errorf("failed to insert multiple rows to the dependency table, sample row %d, with err: %v", putErr.RowIndex, putErr.Error())
			}
		}
		return err
	}

	return nil
}

// AddDownstreamService add a downstream service dependency of a service.
func (b Backend) AddDownstreamService(ctx context.Context, downstreamService federator.DownstreamService, timeOfDataPoint time.Time) error {
	localProcessName := bigquery.NullString{}
	if downstreamService.LocalProcessName != "" {
		localProcessName.StringVal = downstreamService.LocalAddress
	}
	localAddress := bigquery.NullString{}
	if downstreamService.LocalAddress != "" {
		localAddress.StringVal = downstreamService.LocalAddress
	}
	remoteAddress := bigquery.NullString{}
	if downstreamService.DownstreamAddress != "" {
		remoteAddress.StringVal = downstreamService.DownstreamAddress
	}
	localPort := bigquery.NullString{}
	if downstreamService.LocalPort != "" {
		localPort.StringVal = downstreamService.LocalPort
	}

	data := DependencyTableSchema{
		InventoryDate: civil.DateTimeOf(timeOfDataPoint),

		DependencyDirection:       downstreamDependencyDirection,
		Protocol:                  downstreamService.Protocol,
		LocalHostgroupProcessName: localProcessName,

		LocalHostgroupAddressPort: localPort,

		LocalHostgroup:        downstreamService.LocalHostgroup,
		LocalHostgroupAddress: localAddress,

		RemoteHostgroup:        downstreamService.DownstreamHostgroup,
		RemoteHostgroupAddress: remoteAddress,

		// This is null for a downstream dependency data
		RemoteHostgroupAddressPort: bigquery.NullString{},
	}

	inserter := b.dependencyTable.Inserter()
	err := inserter.Put(ctx, data)
	if err != nil {
		if multiErr, ok := err.(bigquery.PutMultiError); ok {
			for _, putErr := range multiErr {
				// Return first one as a hint for the cause of error.
				return fmt.Errorf("failed to insert multiple rows to the dependency table, sample row %d, with err: %v", putErr.RowIndex, putErr.Error())
			}
		}
		return err
	}

	return nil
}

// Flush any outstanding inserts.
func (b Backend) Flush() {
	return
}
