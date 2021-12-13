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
	"io"
	"io/ioutil"
	"net/http"
	"planet-exporter/pkg/network"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

// task that queries inventory data and aggregates them into usable mapping table
type task struct {
	enabled         bool
	inventoryAddr   string
	inventoryFormat string

	mu         sync.Mutex
	values     map[string]Host
	httpClient *http.Client
}

var (
	once      sync.Once
	singleton task

	supportedInventoryFormats = map[string]bool{
		"arrayjson": true,
		"ndjson":    true,
	}
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
func InitTask(ctx context.Context, enabled bool, inventoryAddr string, inventoryFormat string) {
	// Validate inventory format
	if _, ok := supportedInventoryFormats[inventoryFormat]; !ok {
		log.Warningf("Unsupported inventory format '%v', fallback to the default format", inventoryFormat)
		inventoryFormat = "arrayjson"
	}
	log.Infof("Using inventory format '%v'", inventoryFormat)

	once.Do(func() {
		singleton.enabled = enabled
		singleton.inventoryAddr = inventoryAddr
		singleton.inventoryFormat = inventoryFormat
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

	metrics, err := parseInventory(singleton.inventoryFormat, response.Body)
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

func parseInventory(format string, inventoryData io.ReadCloser) ([]Host, error) {
	var result []Host

	decoder := json.NewDecoder(inventoryData)
	decoder.DisallowUnknownFields()

	switch format {
	case "ndjson":
		inventoryEntry := Host{}
		for decoder.More() {
			err := decoder.Decode(&inventoryEntry)
			if err != nil {
				log.Errorf("Skip an inventory entry due to parser error: %v", err)
				continue
			}
			result = append(result, inventoryEntry)
		}

	case "arrayjson":
		err := decoder.Decode(&result)
		if err != nil {
			return nil, err
		}

		// We expect a single JSON array object here. Clear unexpected data that's left in the io.ReadCloser
		if decoder.More() {
			bytesCopied, _ := io.Copy(ioutil.Discard, inventoryData)
			log.Warnf("Unexpected remaining data (%v Bytes) in inventory response: %v", bytesCopied, singleton.inventoryAddr)
		}
	}
	log.Debugf("Parsed %v inventory data", len(result))

	return result, nil
}
