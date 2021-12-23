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
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/prom2json"
)

// TODO: Complete package
// e.g. abstract prom2json data structures and maybe share http client

// Client for Prometheus endpoints.
type Client struct {
	httpTransport *http.Transport
}

// New Prometheus client used to consume Prometheus metrics endpoints.
func New(httpTransport *http.Transport) *Client {
	if httpTransport == nil {
		// Use sane defaults from http.DefaultTransport
		httpTransport = &http.Transport{ // nolint:exhaustivestruct
			DialContext: (&net.Dialer{ // nolint:exhaustivestruct
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			TLSClientConfig:       &tls.Config{InsecureSkipVerify: true}, // nolint:gosec,exhaustivestruct
			ExpectContinueTimeout: 1 * time.Second,
		}
	}

	return &Client{
		httpTransport: httpTransport,
	}
}

// Scrape metrics from a Prometheus HTTP endpoint.
func (c *Client) Scrape(ctx context.Context, url string) ([]*prom2json.Family, error) {
	var err error
	const metricsFamiliesCapacity = 1024
	mfChan := make(chan *dto.MetricFamily, metricsFamiliesCapacity)

	if err != nil {
		return nil, err
	}
	err = prom2json.FetchMetricFamilies(url, mfChan, c.httpTransport)
	if err != nil {
		return nil, fmt.Errorf("error fetching metric families: %w", err)
	}

	result := []*prom2json.Family{}
	for mf := range mfChan {
		result = append(result, prom2json.NewFamily(mf))
	}

	return result, nil
}
