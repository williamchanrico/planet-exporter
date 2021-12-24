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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/prom2json"
	"github.com/stretchr/testify/assert"
)

func TestClient_Scrape(t *testing.T) {
	// nolint:lll
	mockScrapeResponse := `
# HELP test_metric A metric for unit-test.
# TYPE test_metric gauge
test_metric{label_a="a",label_b="b"} 1

# HELP planet_upstream Upstream dependency of this machine
# TYPE planet_upstream gauge
planet_upstream{local_address="debugapp.service.consul",local_hostgroup="debugapp",port="80",process_name="debugapp",protocol="tcp",remote_address="xyz.service.consul",remote_hostgroup="xyz"} 1
planet_upstream{local_address="debugapp.service.consul",local_hostgroup="debugapp",port="8500",process_name="consul-template",protocol="tcp",remote_address="127.0.0.1",remote_hostgroup="localhost"} 1

# HELP request_duration Time for HTTP request.
# TYPE request_duration histogram
request_duration_bucket{le="0.005",} 0.0
request_duration_bucket{le="0.01",} 0.0
request_duration_bucket{le="0.025",} 0.0
request_duration_bucket{le="0.05",} 0.0
request_duration_bucket{le="0.075",} 0.0
request_duration_bucket{le="0.1",} 0.0
request_duration_bucket{le="0.25",} 0.0
request_duration_bucket{le="0.5",} 0.0
request_duration_bucket{le="0.75",} 0.0
request_duration_bucket{le="1.0",} 0.0
request_duration_bucket{le="2.5",} 0.0
request_duration_bucket{le="5.0",} 1.0
request_duration_bucket{le="7.5",} 1.0
request_duration_bucket{le="10.0",} 3.0
request_duration_bucket{le="+Inf",} 3.0
request_duration_count 3.0
request_duration_sum 22.978489699999997
	`

	mockhttpserver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, mockScrapeResponse)
	}))
	defer mockhttpserver.Close()

	type fields struct {
		httpTransport *http.Transport
	}
	type args struct {
		ctx context.Context
		url string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []*prom2json.Family
		wantErr bool
	}{
		{
			name: "Scrape multiple Prometheus metrics",
			fields: fields{
				&http.Transport{}, // nolint:exhaustivestruct
			},
			args: args{
				ctx: context.Background(),
				url: mockhttpserver.URL,
			},
			want: []*prom2json.Family{
				{
					Name: "test_metric",
					Help: "A metric for unit-test.",
					Type: "GAUGE",
					Metrics: []interface{}{
						prom2json.Metric{
							Labels: map[string]string{
								"label_a": "a",
								"label_b": "b",
							},
							TimestampMs: "",
							Value:       "1",
						},
					},
				},
				{
					Name: "planet_upstream",
					Help: "Upstream dependency of this machine",
					Type: "GAUGE",
					Metrics: []interface{}{
						prom2json.Metric{
							// nolint:lll
							Labels: map[string]string{
								"local_address": "debugapp.service.consul", "local_hostgroup": "debugapp", "port": "80", "process_name": "debugapp", "protocol": "tcp", "remote_address": "xyz.service.consul", "remote_hostgroup": "xyz",
							},
							TimestampMs: "",
							Value:       "1",
						},
						prom2json.Metric{
							// nolint:lll
							Labels: map[string]string{
								"local_address": "debugapp.service.consul", "local_hostgroup": "debugapp", "port": "8500", "process_name": "consul-template", "protocol": "tcp", "remote_address": "127.0.0.1", "remote_hostgroup": "localhost",
							},
							TimestampMs: "",
							Value:       "1",
						},
					},
				},
				{
					Name: "request_duration",
					Help: "Time for HTTP request.",
					Type: "HISTOGRAM",
					Metrics: []interface{}{
						prom2json.Histogram{
							Labels:      map[string]string{},
							TimestampMs: "",
							Buckets: map[string]string{
								"+Inf":  "3",
								"0.005": "0",
								"0.01":  "0",
								"0.025": "0",
								"0.05":  "0",
								"0.075": "0",
								"0.1":   "0",
								"0.25":  "0",
								"0.5":   "0",
								"0.75":  "0",
								"1":     "0",
								"10":    "3",
								"2.5":   "0",
								"5":     "1",
								"7.5":   "1",
							},
							Count: "3",
							Sum:   "22.978489699999997",
						},
					},
				},
			},
		},
	}

	assert := assert.New(t)

	for _, testcase := range tests {
		t.Run(testcase.name, func(t *testing.T) {
			c := New(testcase.fields.httpTransport)
			got, err := c.Scrape(testcase.args.ctx, testcase.args.url)
			if (err != nil) != testcase.wantErr {
				t.Errorf("Client.Scrape() error = %v, wantErr %v", err, testcase.wantErr)

				return
			}
			assert.ElementsMatch(got, testcase.want)
		})
	}
}
