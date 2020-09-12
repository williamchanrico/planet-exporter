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

package inventory

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gojektech/heimdall/v6/httpclient"
	log "github.com/sirupsen/logrus"
)

type task struct {
	enabled bool
	inventoryAddr string

	mu      sync.Mutex
	values  map[string]Host
}

var once sync.Once
var singleton task

func init() {
	singleton = task{
		enabled: false,
		mu:      sync.Mutex{},
		values:  make(map[string]Host),
	}
}

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

// Get returns latest metrics in cache.values
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

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	timeout := 5000 * time.Millisecond
	client := httpclient.NewClient(httpclient.WithHTTPTimeout(timeout))

	res, err := client.Get(singleton.inventoryAddr, nil)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	var metrics []Host
	decoder := json.NewDecoder(res.Body)
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
	log.Debugf("taskinventory.Collect process took %v", time.Now().Sub(startTime))
	return nil
}
