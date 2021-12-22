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

package socketstat

import (
	"context"
	"fmt"
	"planet-exporter/collector/task/inventory"
	"planet-exporter/pkg/network"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

// task that queries local socket info and aggregates them into usable planet metrics
type task struct {
	enabled bool

	serverProcesses []Process
	upstreams       []Connections
	downstreams     []Connections
	mu              sync.Mutex
}

var (
	singleton task
)

func init() {
	singleton = task{
		serverProcesses: []Process{},
		upstreams:       []Connections{},
		downstreams:     []Connections{},
		enabled:         false,
		mu:              sync.Mutex{},
	}
}

// InitTask initial states
func InitTask(ctx context.Context, enabled bool) {
	singleton.enabled = enabled
}

// Process that binds on one or more network interfaces
type Process struct {
	Name string // e.g. "node_exporter"
	Bind string // e.g. "0.0.0.0:9100"
	Port string // e.g. "9100"
}

// Connections socket connection metrics
type Connections struct {
	LocalHostgroup  string
	LocalAddress    string
	RemoteHostgroup string
	RemoteAddress   string
	Port            string
	Protocol        string // tcp/udp
	ProcessName     string
}

// Get returns latest metrics from singleton
func Get() ([]Process, []Connections, []Connections) {
	singleton.mu.Lock()
	up := singleton.upstreams
	down := singleton.downstreams
	serverProcesses := singleton.serverProcesses
	singleton.mu.Unlock()

	return serverProcesses, up, down
}

// Collect will collect fill singleton with latest data
func Collect(ctx context.Context) error {
	if !singleton.enabled {
		return nil
	}

	startTime := time.Now()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Get server and peers connections
	serverConnectionStat, err := network.ServerConnections(ctx)
	if err != nil {
		return err
	}

	// Listening server processes
	serverProcesses := []Process{}

	// Listening server ports
	listeningPortsConns := make(map[uint32]network.ListeningConnSocket)

	// Iterate over connection sockets that are in LISTEN state
	for _, v := range serverConnectionStat.ListeningConnSockets {
		// Build serverProcesses from server LISTEN sockets
		serverProcesses = append(serverProcesses, Process{
			Name: v.ProcessName,
			Bind: fmt.Sprintf("%v:%v", v.LocalIP, v.LocalPort),
			Port: fmt.Sprint(v.LocalPort),
		})

		// Build list of listening server ports from server LISTEN sockets
		listeningPortsConns[v.LocalPort] = v
		log.Debugf("Server listening on: %v:%v [process:%v]", v.LocalIP, v.LocalPort, v.ProcessName)
	}

	inventoryHosts := inventory.Get()

	// Find current IP to replace loop-back address
	currentIP, err := network.LocalIP()
	if err != nil {
		return err
	}

	exists := make(map[string]bool)

	// Build upstreams and downstreams from every peered connection sockets (e.g. "ss -pant")
	var upstreams []Connections
	var downstreams []Connections
	for _, peeredConn := range serverConnectionStat.PeeredConnSockets {
		// Replace localhost or 127.0.0.1 with a more useful current address
		if peeredConn.LocalIP == "127.0.0.1" {
			peeredConn.LocalIP = currentIP.String()
		}

		// Find local Host inventory
		// This should be the same most of the time,
		// but we find LocalIP's inventory for every peeredConn in case there's interface address spoofing.
		var localAddr, localHostgroup string
		if localInventoryHost, foundInventory := inventoryHosts.GetHost(peeredConn.LocalIP); foundInventory {
			localAddr = localInventoryHost.Domain
			localHostgroup = localInventoryHost.Hostgroup
		}
		if localAddr == "" {
			localAddr = peeredConn.LocalIP
		}

		// Find remote Host inventory
		var remoteAddr, remoteHostgroup string
		if remoteInventoryHost, foundInventory := inventoryHosts.GetHost(peeredConn.RemoteIP); foundInventory {
			remoteAddr = remoteInventoryHost.Domain
			remoteHostgroup = remoteInventoryHost.Hostgroup

		}
		if remoteAddr == "" {
			remoteAddr = peeredConn.RemoteIP
		}

		// Check whether this is a downstream/upstream connection tuple
		if listeningConn, foundListeningConn := listeningPortsConns[peeredConn.LocalPort]; foundListeningConn {
			// It's a downstream connection. The peerConn.localPort is one of the listening port.

			// To track whether we have considered this downstream connection
			existenceKey := fmt.Sprintf("down_%s_%s_%v_%s", remoteHostgroup, remoteAddr, peeredConn.LocalPort, peeredConn.Protocol)

			// Prevents duplicate downstream conn entries
			if _, ok := exists[existenceKey]; !ok {

				// Empty process name on a connection socket usually comes from TIME_WAIT state, they don't have PID anymore.
				// Since we know it's a conn coming to listening port, we set process name to the server process that's listening on that port.
				if peeredConn.ProcessName == "" {
					peeredConn.ProcessName = listeningConn.ProcessName
				}

				downstreams = append(downstreams, Connections{
					LocalHostgroup:  localHostgroup,
					RemoteHostgroup: remoteHostgroup,
					LocalAddress:    localAddr,
					RemoteAddress:   remoteAddr,
					Port:            fmt.Sprint(peeredConn.LocalPort),
					Protocol:        peeredConn.Protocol,
					ProcessName:     peeredConn.ProcessName,
				})
				exists[existenceKey] = true
			}

		} else if remoteAddr != "localhost" {
			// It's an upstream connection otherwise.

			remotePort := fmt.Sprint(peeredConn.RemotePort)
			existenceKey := fmt.Sprintf("up_%s_%s_%s_%s", remoteHostgroup, remoteAddr, remotePort, peeredConn.Protocol)

			if _, ok := exists[existenceKey]; !ok {
				upstreams = append(upstreams, Connections{
					LocalHostgroup:  localHostgroup,
					RemoteHostgroup: remoteHostgroup,
					LocalAddress:    localAddr,
					RemoteAddress:   remoteAddr,
					Port:            remotePort,
					Protocol:        peeredConn.Protocol,
					ProcessName:     peeredConn.ProcessName,
				})
				exists[existenceKey] = true
			}
		}
	}

	singleton.mu.Lock()
	singleton.serverProcesses = serverProcesses
	singleton.upstreams = upstreams
	singleton.downstreams = downstreams
	singleton.mu.Unlock()

	log.Debugf("tasksocketstat.Collect retrieved %v upstreams metrics", len(upstreams))
	log.Debugf("tasksocketstat.Collect retrieved %v downstreams metrics", len(downstreams))
	log.Debugf("tasksocketstat.Collect process took %v", time.Since(startTime))
	return nil
}
