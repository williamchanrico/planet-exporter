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

package inventory

import (
	"io"
	"reflect"
	"strings"
	"testing"
)

// mockInventoryResponse returns an io.ReadCloser simulating data coming from an inventory endpoint
func mockInventoryResponse(raw string) io.ReadCloser {
	return io.NopCloser(strings.NewReader(raw))
}

func Test_parseInventory(t *testing.T) {
	type args struct {
		inventoryFormat string
		inventoryData   io.ReadCloser
	}

	tests := []struct {
		name    string
		args    args
		want    []Host
		wantErr bool
	}{
		// Format: 'ndjson'
		{
			name: "Test single ndjson inventory entry",
			args: args{
				inventoryFormat: "ndjson",
				inventoryData: mockInventoryResponse(`
					{"ip_address":"10.0.1.2","domain":"xyz.service.consul","hostgroup":"xyz"}
				`),
			},
			want: []Host{
				{IPAddress: "10.0.1.2", Domain: "xyz.service.consul", Hostgroup: "xyz"},
			},
		},
		{
			name: "Test multiple ndjson inventory entries",
			args: args{
				inventoryFormat: "ndjson",
				inventoryData: mockInventoryResponse(`
					{"ip_address":"10.0.1.2","domain":"xyz.service.consul","hostgroup":"xyz"}
					{"ip_address":"172.16.1.2","domain":"abc.service.consul","hostgroup":"abc"}
				`),
			},
			want: []Host{
				{IPAddress: "10.0.1.2", Domain: "xyz.service.consul", Hostgroup: "xyz"},
				{IPAddress: "172.16.1.2", Domain: "abc.service.consul", Hostgroup: "abc"},
			},
		},

		// Format: 'arrayjson'
		{
			name: "Test single arrayjson inventory entry",
			args: args{
				inventoryFormat: "arrayjson",
				inventoryData: mockInventoryResponse(`
					[
						{"ip_address":"10.0.1.2","domain":"xyz.service.consul","hostgroup":"xyz"}
					]
				`),
			},
			want: []Host{
				{IPAddress: "10.0.1.2", Domain: "xyz.service.consul", Hostgroup: "xyz"},
			},
		},
		{
			name: "Test multiple arrayjson inventory entries",
			args: args{
				inventoryFormat: "arrayjson",
				inventoryData: mockInventoryResponse(`
					[
						{"ip_address":"10.0.1.2","domain":"xyz.service.consul","hostgroup":"xyz"},
						{"ip_address":"172.16.1.2","domain":"abc.service.consul","hostgroup":"abc"}
					]
				`),
			},
			want: []Host{
				{IPAddress: "10.0.1.2", Domain: "xyz.service.consul", Hostgroup: "xyz"},
				{IPAddress: "172.16.1.2", Domain: "abc.service.consul", Hostgroup: "abc"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseInventory(tt.args.inventoryFormat, tt.args.inventoryData)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseInventory() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseInventory() = %v, want %v", got, tt.want)
			}
		})
	}
}
