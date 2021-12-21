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

// task that queries inventory data and aggregates them into usable mapping table
type task struct {
	enabled         bool
	inventoryAddr   string
	inventoryFormat string

	mu         sync.Mutex
	values     Inventory
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
		values: Inventory{
			ipToHosts:      make(map[string]Host),
			networkToHosts: []ipNetHost{},
		},
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

// Get returns latest metrics from cache.values
func Get() Inventory {
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

// ipNetHost represents a mapping between network address to Host info
type ipNetHost struct {
	network *net.IPNet
	host    Host
}

// Inventory contains mappings to Host information
type Inventory struct {
	// ipToHosts maps between IP to Host info
	ipToHosts map[string]Host
	// networkToHosts maps between network (in CIDR notation) to Host info
	networkToHosts []ipNetHost
}

// GetHost returns a Host information based on IP or Network address, in that order.
func (i Inventory) GetHost(address string) (Host, bool) {
	// Priority 1: Check direct single IP address match for the address
	if host, ok := i.ipToHosts[address]; ok {
		return host, true
	}

	// Priority 2: Check for longest-prefix match with targetIP
	targetIP := net.ParseIP(address)
	matchedHost := Host{}
	matchedPrefixLen := -1
	for _, ipNetHost := range i.networkToHosts {
		currPrefixLen, _ := ipNetHost.network.Mask.Size()
		if ipNetHost.network.Contains(targetIP) && currPrefixLen > matchedPrefixLen {
			matchedPrefixLen = currPrefixLen
			matchedHost = ipNetHost.host
		}
	}
	// There is a match when it's greater than 0 (even 0.0.0.0/0)
	if matchedPrefixLen >= 0 {
		return matchedHost, true
	}

	return Host{}, false
}

// parseInventory parses a list of Host into an Inventory
// This function supports hosts with IP address containing "/" (CIDR notation).
func parseInventory(hosts []Host) Inventory {
	ipToHosts := make(map[string]Host)
	networkToHosts := []ipNetHost{}
	for _, host := range hosts {
		// Skip unknown hosts as they provide zero value for Planet Exporter
		if host.Domain == "" && host.Hostgroup == "" {
			continue
		}

		// For CIDR notation address, we put the mapping in a list of ipNetHost
		if strings.Contains(host.IPAddress, "/") {
			_, network, err := net.ParseCIDR(host.IPAddress)
			if err != nil {
				log.Debugf("Failed to parse CIDR address from an inventory host entry (address=%v): %v", host.IPAddress, err)
				continue
			}
			networkToHost := ipNetHost{
				network: network,
				host:    host,
			}
			networkToHosts = append(networkToHosts, networkToHost)
		} else {
			// Standard mapping between IP and Host
			ipToHosts[host.IPAddress] = host
		}

	}

	return Inventory{
		ipToHosts:      ipToHosts,
		networkToHosts: networkToHosts,
	}
}

// GetLocalInventory returns current Host's inventory entry
func GetLocalInventory() Host {
	localHost := Host{}
	defaultLocalAddr, err := network.DefaultLocalAddr()
	if err != nil {
		return localHost
	}

	inventory := Get()

	if h, ok := inventory.GetHost(defaultLocalAddr.String()); ok {
		localHost.IPAddress = h.IPAddress
		localHost.Domain = h.Domain
		localHost.Hostgroup = h.Hostgroup
	}

	return localHost
}
