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
	"net"
	"reflect"
	"strings"
	"testing"
)

// mockHostsResponseData returns an io.Reader simulating inventory JSON data returned from upstream
func mockHostsResponseData(raw string) io.ReadCloser {
	return io.NopCloser(strings.NewReader(raw))
}

func Test_parseHosts(t *testing.T) {
	type args struct {
		format string
		data   io.Reader
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
				format: "ndjson",
				data: mockHostsResponseData(`
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
				format: "ndjson",
				data: mockHostsResponseData(`
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
				format: "arrayjson",
				data: mockHostsResponseData(`
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
				format: "arrayjson",
				data: mockHostsResponseData(`
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
			got, err := parseHosts(tt.args.format, tt.args.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseHosts() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseHosts() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_parseInventory(t *testing.T) {
	_, exampleCIDRNetwork, _ := net.ParseCIDR("10.1.0.0/16")
	_, exampleCIDRNetworkQuadZero, _ := net.ParseCIDR("0.0.0.0/0")

	type args struct {
		hosts []Host
	}
	tests := []struct {
		name string
		args args
		want Inventory
	}{
		{
			name: "Single host inventory",
			args: args{
				hosts: []Host{
					{Domain: "unit-test.local", IPAddress: "1.2.3.4", Hostgroup: "unit-test"},
				},
			},
			want: Inventory{
				ipAddresses: map[string]Host{
					"1.2.3.4": {Domain: "unit-test.local", IPAddress: "1.2.3.4", Hostgroup: "unit-test"},
				},
				networkCIDRAddresses: []networkHost{},
			},
		},
		{
			name: "Multiple hosts inventory",
			args: args{
				hosts: []Host{
					{Domain: "unit-test-1.local", IPAddress: "1.2.3.4", Hostgroup: "unit-test"},
					{Domain: "unit-test-2.local", IPAddress: "1.2.3.5", Hostgroup: "unit-test"},
					{Domain: "unit-test-3.local", IPAddress: "1.2.3.6", Hostgroup: "unit-test"},
				},
			},
			want: Inventory{
				ipAddresses: map[string]Host{
					"1.2.3.4": {Domain: "unit-test-1.local", IPAddress: "1.2.3.4", Hostgroup: "unit-test"},
					"1.2.3.5": {Domain: "unit-test-2.local", IPAddress: "1.2.3.5", Hostgroup: "unit-test"},
					"1.2.3.6": {Domain: "unit-test-3.local", IPAddress: "1.2.3.6", Hostgroup: "unit-test"},
				},
				networkCIDRAddresses: []networkHost{},
			},
		},
		{
			name: "Skip unknown inventory host when both Domain and Hostgroup are empty",
			args: args{
				hosts: []Host{
					{Domain: "unit-test.local", IPAddress: "1.2.3.4", Hostgroup: "unit-test"},
					{Domain: "", IPAddress: "1.2.3.5", Hostgroup: ""},
				},
			},
			want: Inventory{
				ipAddresses: map[string]Host{
					"1.2.3.4": {Domain: "unit-test.local", IPAddress: "1.2.3.4", Hostgroup: "unit-test"},
				},
				networkCIDRAddresses: []networkHost{},
			},
		},
		{
			name: "Inventory host with CIDR notation address is parsed",
			args: args{
				hosts: []Host{
					{Domain: "unit-test.local", IPAddress: "1.2.3.4", Hostgroup: "unit-test"},
					{Domain: "unit-test-cidr.local", IPAddress: exampleCIDRNetwork.String(), Hostgroup: "unit-test-cidr"}, // e.g. 10.1.0.0/16
				},
			},
			want: Inventory{
				ipAddresses: map[string]Host{
					"1.2.3.4": {Domain: "unit-test.local", IPAddress: "1.2.3.4", Hostgroup: "unit-test"},
				},
				networkCIDRAddresses: []networkHost{
					{
						network: exampleCIDRNetwork,
						host:    Host{Domain: "unit-test-cidr.local", IPAddress: exampleCIDRNetwork.String(), Hostgroup: "unit-test-cidr"},
					},
				},
			},
		},
		{
			name: "Inventory host with invalid CIDR notation address is skipped",
			args: args{
				hosts: []Host{
					{Domain: "unit-test.local", IPAddress: "1.2.3.4", Hostgroup: "unit-test"},
					{Domain: "unit-test-cidr.local", IPAddress: exampleCIDRNetwork.String(), Hostgroup: "unit-test-cidr"}, // e.g. 10.1.0.0/16
					{Domain: "unit-test-cidr.local", IPAddress: "100.100.100.100/100", Hostgroup: "unit-test-cidr"},       // Invalid
				},
			},
			want: Inventory{
				ipAddresses: map[string]Host{
					"1.2.3.4": {Domain: "unit-test.local", IPAddress: "1.2.3.4", Hostgroup: "unit-test"},
				},
				networkCIDRAddresses: []networkHost{
					{
						network: exampleCIDRNetwork,
						host:    Host{Domain: "unit-test-cidr.local", IPAddress: exampleCIDRNetwork.String(), Hostgroup: "unit-test-cidr"},
					},
				},
			},
		},
		{
			name: "Inventory host with CIDR notation /0 is valid",
			args: args{
				hosts: []Host{
					{Domain: "unit-test.local", IPAddress: "1.2.3.4", Hostgroup: "unit-test"},
					{Domain: "unit-test-cidr.local", IPAddress: exampleCIDRNetworkQuadZero.String(), Hostgroup: "unit-test-cidr"}, // e.g. 0.0.0.0/0
				},
			},
			want: Inventory{
				ipAddresses: map[string]Host{
					"1.2.3.4": {Domain: "unit-test.local", IPAddress: "1.2.3.4", Hostgroup: "unit-test"},
				},
				networkCIDRAddresses: []networkHost{
					{
						network: exampleCIDRNetworkQuadZero,
						host:    Host{Domain: "unit-test-cidr.local", IPAddress: exampleCIDRNetworkQuadZero.String(), Hostgroup: "unit-test-cidr"},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseInventory(tt.args.hosts); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseInventory() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInventory_GetHost(t *testing.T) {
	type fields struct {
		ipAddresses          map[string]Host
		networkCIDRAddresses []networkHost
	}
	type args struct {
		address string
	}

	// Prepare test data
	_, exampleCIDR1, _ := net.ParseCIDR("10.0.0.0/17")
	_, exampleCIDR2, _ := net.ParseCIDR("10.0.32.0/19")
	_, exampleCIDRQuadZero, _ := net.ParseCIDR("0.0.0.0/0")
	inventory := fields{
		ipAddresses: map[string]Host{
			"1.2.3.4": {Hostgroup: "unit-test", IPAddress: "1.2.3.4", Domain: "unit-test.local"},
			"1.2.3.5": {Hostgroup: "unit-test", IPAddress: "1.2.3.5", Domain: "unit-test.local"},
		},
		networkCIDRAddresses: []networkHost{
			{network: exampleCIDR1, host: Host{Hostgroup: "unit-test-cidr-1", IPAddress: exampleCIDR1.String(), Domain: "unit-test-cidr-1.local"}},
			{network: exampleCIDR2, host: Host{Hostgroup: "unit-test-cidr-2", IPAddress: exampleCIDR2.String(), Domain: "unit-test-cidr-2.local"}},
			{network: exampleCIDRQuadZero, host: Host{Hostgroup: "unit-test-cidr-quad-zero", IPAddress: exampleCIDRQuadZero.String(), Domain: "unit-test-cidr-quad-zero.local"}},
		},
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		want1  Host
		want2  bool
	}{
		{
			name:   "Simple IP match",
			fields: inventory,
			args:   args{address: "1.2.3.4"},
			want1:  Host{Hostgroup: "unit-test", IPAddress: "1.2.3.4", Domain: "unit-test.local"},
			want2:  true,
		},
		{
			name:   "Network CIDR match",
			fields: inventory,
			args:   args{address: "10.0.31.1"},
			want1:  Host{Hostgroup: "unit-test-cidr-1", IPAddress: exampleCIDR1.String(), Domain: "unit-test-cidr-1.local"},
			want2:  true,
		},
		{
			name:   "Longest-prefix Network CIDR match",
			fields: inventory,
			args:   args{address: "10.0.32.1"},
			want1:  Host{Hostgroup: "unit-test-cidr-2", IPAddress: exampleCIDR2.String(), Domain: "unit-test-cidr-2.local"},
			want2:  true,
		},
		{
			name:   "Always match a 0.0.0.0/0",
			fields: inventory,
			args:   args{address: "123.123.123.123"},
			want1:  Host{Hostgroup: "unit-test-cidr-quad-zero", IPAddress: exampleCIDRQuadZero.String(), Domain: "unit-test-cidr-quad-zero.local"},
			want2:  true,
		},
		{
			name: "No match returns empty Host",
			fields: fields{
				ipAddresses:          make(map[string]Host),
				networkCIDRAddresses: []networkHost{},
			},
			args:  args{address: "123.123.123.123"},
			want1: Host{},
			want2: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := Inventory{
				ipAddresses:          tt.fields.ipAddresses,
				networkCIDRAddresses: tt.fields.networkCIDRAddresses,
			}
			got1, got2 := i.GetHost(tt.args.address)
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("Inventory.GetHost() got1 = %v, want %v", got1, tt.want1)
			}
			if got2 != tt.want2 {
				t.Errorf("Inventory.GetHost() got2 = %v, want %v", got2, tt.want2)
			}
		})
	}
}
