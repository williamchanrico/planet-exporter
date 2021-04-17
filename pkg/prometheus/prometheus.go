// Copyright 2020 - williamchanrico@gmail.com
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
	"crypto/tls"
	"net/http"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/prom2json"
)

// TODO: Complete package
// e.g. abstract prom2json data structures

// Scrape metrics from a prometheus HTTP endpoint
func Scrape(url string) ([]*prom2json.Family, error) {
	var err error

	mfChan := make(chan *dto.MetricFamily, 1024)

	transport, err := makeTransport()
	if err != nil {
		return nil, err
	}
	err = prom2json.FetchMetricFamilies(url, mfChan, transport)
	if err != nil {
		return nil, err
	}

	result := []*prom2json.Family{}
	for mf := range mfChan {
		result = append(result, prom2json.NewFamily(mf))
	}

	return result, nil
}

func makeTransport() (*http.Transport, error) {
	var transport *http.Transport
	transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return transport, nil
}
