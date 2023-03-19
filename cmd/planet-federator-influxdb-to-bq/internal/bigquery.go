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

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"
	log "github.com/sirupsen/logrus"
)

// backend interface for a time-series DB handling pre-processed planet-exporter data.
type backend struct {
	client *bigquery.Client

	trafficTable    *bigquery.Table
	dependencyTable *bigquery.Table
}

// TableMetadata represents a BigQuery Table Metadata.
type TableMetadata struct {
	DatasetID string
	TableID   string
}

// newBackend returns new BigQuery storage client.
func newBackend(config Config, bqClient *bigquery.Client) backend {
	trafficTable := bqClient.Dataset(config.BigqueryDatasetID).Table(config.BigqueryTrafficTableID)
	dependencyTable := bqClient.Dataset(config.BigqueryDatasetID).Table(config.BigqueryDependencyTableID)

	return backend{
		client:          bqClient,
		trafficTable:    trafficTable,
		dependencyTable: dependencyTable,
	}
}

const (
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
//         "name": "traffic_bandwidth_bits_min_1h",
//         "type": "INTEGER",
//         "mode": "REQUIRED",
//         "description": "The 1h min traffic bandwidth consumed in bit per second."
//     },
//     {
//         "name": "traffic_bandwidth_bits_max_1h",
//         "type": "INTEGER",
//         "mode": "REQUIRED",
//         "description": "The 1h max traffic bandwidth consumed in bit per second."
//     },
//     {
//         "name": "traffic_bandwidth_bits_avg_1h",
//         "type": "INTEGER",
//         "mode": "REQUIRED",
//         "description": "The 1h avg traffic bandwidth consumed in bit per second."
//     }
// ]

// TrafficTableSchema represents the schema for traffic table.
type TrafficTableSchema struct {
	InventoryDate             civil.DateTime      `bigquery:"inventory_date"`
	TrafficDirection          string              `bigquery:"traffic_direction"`
	LocalHostgroup            string              `bigquery:"local_hostgroup"`
	LocalHostgroupAddress     bigquery.NullString `bigquery:"local_hostgroup_address"`
	RemoteHostgroup           string              `bigquery:"remote_hostgroup"`
	RemoteHostgroupAddress    bigquery.NullString `bigquery:"remote_hostgroup_address"`
	TrafficBandwidthBitsMin1h int64               `bigquery:"traffic_bandwidth_bits_min_1h"`
	TrafficBandwidthBitsMax1h int64               `bigquery:"traffic_bandwidth_bits_max_1h"`
	TrafficBandwidthBitsAvg1h int64               `bigquery:"traffic_bandwidth_bits_avg_1h"`
}

// InsertTrafficBandwidthData inserts traffic data.
func (b backend) InsertTrafficBandwidthData(ctx context.Context, data []TrafficTableSchema) error {
	inserter := b.trafficTable.Inserter()
	log.Debugf("InsertTrafficBandwidthData inserter.Put len(data)=%v", len(data))
	err := inserter.Put(ctx, data)
	if err != nil {
		if multiErr, ok := err.(bigquery.PutMultiError); ok {
			for _, putErr := range multiErr {
				return fmt.Errorf("failed to insert traffic table, sample row %d, with err: %v", putErr.RowIndex, putErr.Error())
			}
		} else {
			return fmt.Errorf("failed to insert traffic table, with err: %v", err)
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
	InventoryDate civil.DateTime `bigquery:"inventory_date"`

	// DependencyDirection determines whether it's an upstream/downstream dependency.
	DependencyDirection       string              `bigquery:"dependency_direction"`
	Protocol                  string              `bigquery:"protocol"`
	LocalHostgroupProcessName bigquery.NullString `bigquery:"local_hostgroup_process_name"`

	LocalHostgroup        string              `bigquery:"local_hostgroup"`
	LocalHostgroupAddress bigquery.NullString `bigquery:"local_hostgroup_address"`

	// LocalHostgroupPort is only relevant for dependencyDirection=downstream
	// This signifies which local port that the downstream connected to.
	LocalHostgroupAddressPort bigquery.NullString `bigquery:"local_hostgroup_address_port"`

	RemoteHostgroup        string              `bigquery:"remote_hostgroup"`
	RemoteHostgroupAddress bigquery.NullString `bigquery:"remote_hostgroup_address"`

	// RemoteHostgroupPort is only relevant for dependencyDirection=upstream
	// This signifies the upstream port.
	RemoteHostgroupAddressPort bigquery.NullString `bigquery:"remote_hostgroup_address_port"`
}

// InsertDependencyData inserts dependency data.
func (b backend) InsertDependencyData(ctx context.Context, data []DependencyTableSchema) error {
	inserter := b.dependencyTable.Inserter()
	log.Debugf("InsertDependencyData inserter.Put len(data)=%v", len(data))
	err := inserter.Put(ctx, data)
	if err != nil {
		if multiErr, ok := err.(bigquery.PutMultiError); ok {
			for _, putErr := range multiErr {
				return fmt.Errorf("failed to insert multiple rows to the dependency table, sample row %d, with err: %v", putErr.RowIndex, putErr.Error())
			}
		} else {
			return fmt.Errorf("failed to insert dependency table, with err: %v", err)
		}
		return err
	}

	return nil
}
