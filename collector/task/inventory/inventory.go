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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"planet-exporter/pkg/network"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

// task that queries inventory data and aggregates them into usable mapping table
type task struct {
	enabled       bool
	inventoryAddr string

	mu         sync.Mutex
	values     map[string]Host
	httpClient *http.Client
}

var (
	once      sync.Once
	singleton task
)

const (
	collectTimeout = 10 * time.Second
)

func init() {
	singleton = task{
		enabled: false,
		mu:      sync.Mutex{},
		values:  make(map[string]Host),
		httpClient: &http.Client{
			Timeout: collectTimeout,
		},
	}
}

// InitTask initial states
func InitTask(ctx context.Context, enabled bool, inventoryAddr string) {
	once.Do(func() {
		singleton.enabled = enabled
		singleton.inventoryAddr = inventoryAddr
	})
}

// Host contains inventory data
type Host struct {
	Domain    string `json:"domain"`
	Hostgroup string `json:"hostgroup"`
	IPAddress string `json:"ip_address"`
}

// Get returns latest metrics from cache.values
func Get() map[string]Host {
	singleton.mu.Lock()
	hosts := singleton.values
	singleton.mu.Unlock()

	return hosts
}

// Collect will retrieve latest inventory data and fill cache.values with latest data
func Collect(ctx context.Context) error {
	if !singleton.enabled {
		return nil
	}

	if singleton.inventoryAddr == "" {
		return fmt.Errorf("Inventory address is empty")
	}

	startTime := time.Now()

	collectCtx, cancel := context.WithTimeout(ctx, collectTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(collectCtx, http.MethodGet, singleton.inventoryAddr, nil)
	if err != nil {
		return err
	}
	response, err := singleton.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	var metrics []Host
	decoder := json.NewDecoder(response.Body)
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&metrics)
	if err != nil {
		return err
	}

	hosts := make(map[string]Host)
	for _, v := range metrics {
		hosts[v.IPAddress] = v
	}

	singleton.mu.Lock()
	singleton.values = hosts
	singleton.mu.Unlock()

	log.Debugf("taskinventory.Collect retrieved %v hosts", len(hosts))
	log.Debugf("taskinventory.Collect process took %v", time.Since(startTime))
	return nil
}

// GetLocalInventory returns current Host's inventory entry
func GetLocalInventory() Host {
	inv := Host{}
	defaultLocalAddr, err := network.DefaultLocalAddr()
	if err != nil {
		return inv
	}

	hosts := Get()

	singleton.mu.Lock()
	if h, ok := hosts[defaultLocalAddr.String()]; ok {
		inv.IPAddress = h.IPAddress
		inv.Domain = h.Domain
		inv.Hostgroup = h.Hostgroup
	}
	singleton.mu.Unlock()

	return inv
}
