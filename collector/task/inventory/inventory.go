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
	"fmt"
	"net"
	"net/http"
	"planet-exporter/pkg/network"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

// task that queries inventory data and aggregates them into usable inventory
type task struct {
	enabled         bool
	inventoryAddr   string
	inventoryFormat string

	mu         sync.Mutex
	values     Inventory
	httpClient *http.Client
}

const (
	// collectTimeout for inventory requests to upstream
	collectTimeout = 10 * time.Second

	// Inventory formats:
	//   - arrayjson: array of hosts objects '[{},{},{}]'
	//   - ndjson: newline-delimited hosts objects '{}\n{}\n{}'
	fmtArrayJSON string = "arrayjson"
	fmtNDJSON    string = "ndjson"
)

var (
	once      sync.Once
	singleton task

	supportedInventoryFormats = map[string]bool{
		fmtArrayJSON: true,
		fmtNDJSON:    true,
	}
)

func init() {
	singleton = task{
		enabled: false,
		mu:      sync.Mutex{},
		values: Inventory{
			ipAddresses:          make(map[string]Host),
			networkCIDRAddresses: []networkHost{},
		},
		httpClient: &http.Client{
			Timeout: collectTimeout,
		},
	}
}

// InitTask sets initial states
func InitTask(ctx context.Context, enabled bool, inventoryAddr string, inventoryFormat string) {
	// Validate inventory format
	if _, ok := supportedInventoryFormats[inventoryFormat]; !ok {
		log.Warningf("Unsupported inventory format '%v', fallback to the default format", inventoryFormat)
		inventoryFormat = fmtArrayJSON
	}
	log.Infof("Using inventory format '%v'", inventoryFormat)

	once.Do(func() {
		singleton.enabled = enabled
		singleton.inventoryAddr = inventoryAddr
		singleton.inventoryFormat = inventoryFormat
	})
}

// Get returns current inventory data
func Get() Inventory {
	singleton.mu.Lock()
	hosts := singleton.values
	singleton.mu.Unlock()

	return hosts
}

// Collect retrieves real-time inventory data and updates singleton.values
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

	hosts, err := requestHosts(collectCtx, singleton.httpClient, singleton.inventoryFormat, singleton.inventoryAddr)
	if err != nil {
		return err
	}
	hosts = append(hosts, Host{
		IPAddress: "127.0.0.1",
		Domain:    "localhost",
		Hostgroup: "localhost",
	})
	inventory := parseInventory(hosts)

	singleton.mu.Lock()
	singleton.values = inventory
	singleton.mu.Unlock()

	log.Debugf("taskinventory.Collect retrieved %v hosts", len(hosts))
	log.Debugf("taskinventory.Collect process took %v", time.Since(startTime))
	return nil
}

// networkHost represents a mapping of network -> Host info
type networkHost struct {
	network *net.IPNet
	host    Host
}

// Inventory contains mappings to Host information
type Inventory struct {
	// ipAddresses maps IP -> Host info
	ipAddresses map[string]Host
	// networkCIDRAddresses maps network in CIDR notation -> Host info
	networkCIDRAddresses []networkHost
}

// GetHost returns a Host information based on IP or Network address, in that order.
// e.g. address can be "192.168.1.2" or "192.168.0.0/26"
func (i Inventory) GetHost(address string) (Host, bool) {
	// Priority 1: Check for single IP address match for the address within known IP inventory
	if host, ok := i.ipAddresses[address]; ok {
		return host, true
	}

	// Priority 2: Check for longest-prefix match of targetIP within known network CIDR inventory
	targetIP := net.ParseIP(address)
	matchedHost := Host{}
	matchedPrefixLen := -1
	for _, ipNetHost := range i.networkCIDRAddresses {
		currPrefixLen, _ := ipNetHost.network.Mask.Size()
		if ipNetHost.network.Contains(targetIP) && currPrefixLen > matchedPrefixLen {
			matchedPrefixLen = currPrefixLen
			matchedHost = ipNetHost.host
		}
	}
	// There is a match when it's greater than or equal to 0 (even 0.0.0.0/0)
	if matchedPrefixLen >= 0 {
		return matchedHost, true
	}

	return Host{}, false
}

// parseInventory parses a list of Host into an Inventory
// This function supports hosts with IP address containing "/" (CIDR notation).
func parseInventory(hosts []Host) Inventory {
	inventory := Inventory{
		ipAddresses:          make(map[string]Host),
		networkCIDRAddresses: []networkHost{},
	}

	for _, host := range hosts {
		// Skip unknown hosts as they provide zero value for Planet Exporter
		if host.Domain == "" && host.Hostgroup == "" {
			continue
		}

		if strings.Contains(host.IPAddress, "/") {
			// A network CIDR based inventory

			_, network, err := net.ParseCIDR(host.IPAddress)
			if err != nil {
				log.Debugf("Failed to parse CIDR address from an inventory host entry (address=%v): %v", host.IPAddress, err)
				continue
			}
			networkCIDRAddress := networkHost{
				network: network,
				host:    host,
			}

			inventory.networkCIDRAddresses = append(inventory.networkCIDRAddresses, networkCIDRAddress)
		} else {
			// An IP based inventory

			inventory.ipAddresses[host.IPAddress] = host
		}

	}

	return inventory
}

// GetLocalInventory returns an inventory entry for current host
func GetLocalInventory() Host {
	localHost := Host{}
	currentIP, err := network.LocalIP()
	if err != nil {
		return localHost
	}

	inventory := Get()

	if h, ok := inventory.GetHost(currentIP.String()); ok {
		localHost.IPAddress = h.IPAddress
		localHost.Domain = h.Domain
		localHost.Hostgroup = h.Hostgroup
	}

	return localHost
}
