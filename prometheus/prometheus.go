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
	"math"
	"strings"
	"time"

	api "github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	log "github.com/sirupsen/logrus"
)

// https://prometheus.io/docs/prometheus/latest/querying/api/

// Service is prometheus service.
type Service struct {
	promapiClient api.Client
}

// New returns a prometheus client service.
func New(promapiClient api.Client) Service {
	return Service{
		promapiClient: promapiClient,
	}
}

// TODO: Return explicit vector
// nolint:unused
func (s Service) query(ctx context.Context, query string, qTime time.Time) (model.Value, error) {
	const contextTimeoutSeconds = 120
	ctx, cancel := context.WithTimeout(ctx, contextTimeoutSeconds*time.Second)
	defer cancel()

	v1api := v1.NewAPI(s.promapiClient)

	log.Debugf("Query %v", query)
	results, warnings, err := v1api.Query(ctx, query, qTime)
	if err != nil {
		return nil, fmt.Errorf("error on query: %w", err)
	}
	if len(warnings) > 0 {
		for _, v := range warnings {
			log.Warnf("Query %v: %v", query, v)
		}
	}

	return results, nil
}

// TODO: Return explicit matrix.
func (s Service) queryRange(ctx context.Context, query string,
	qStartTime time.Time, qEndTime time.Time) (model.Value, error) {
	const contextTimeoutSeconds = 120
	ctx, cancel := context.WithTimeout(ctx, contextTimeoutSeconds*time.Second)
	defer cancel()

	v1api := v1.NewAPI(s.promapiClient)

	log.Debugf("Query %v", query)
	results, warnings, err := v1api.QueryRange(ctx, query, v1.Range{
		Start: qStartTime,
		End:   qEndTime,
		Step:  1 * time.Minute,
	})
	if err != nil {
		return nil, fmt.Errorf("error on queryRange: %w", err)
	}
	if len(warnings) > 0 {
		for _, v := range warnings {
			log.Warnf("Query %v: %v", query, v)
		}
	}

	return results, nil
}

// ErrMetricsLabelNotFound could not find the label in metrics.
var ErrMetricsLabelNotFound = fmt.Errorf("could not find label in metrics")

// nolint:unused
func (s Service) getLabelValue(label string, metric model.Metric) (string, error) {
	labelValue, ok := metric[model.LabelName(label)]
	if !ok {
		return "", ErrMetricsLabelNotFound
	}

	return string(labelValue), nil
}

// ErrIPAddressMetricsLabelInvalid could not extract the IP address from the metrics.
var ErrIPAddressMetricsLabelInvalid = fmt.Errorf("could not extract IP from the metrics")

// nolint:unused
func (s Service) getIPAddressFromLabelValue(label string, metric model.Metric) (string, error) {
	lvIPAddr, err := s.getLabelValue(label, metric)
	if err != nil {
		return "", err
	}
	ipParts := strings.Split(lvIPAddr, ":")
	if len(ipParts) < 1 {
		return "", ErrIPAddressMetricsLabelInvalid
	}

	return ipParts[0], nil
}

func (s Service) getMaxValueFromSamplePairs(samplePairs []model.SamplePair) float64 {
	maxi := float64(-1)
	for _, v := range samplePairs {
		val := float64(v.Value)
		maxi = math.Max(maxi, val)
	}

	return maxi
}
