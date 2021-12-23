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
	"sync"
	"time"

	"planet-exporter/collector/task/inventory"
	"planet-exporter/pkg/network"

	log "github.com/sirupsen/logrus"
)

// task that queries local socket info and aggregates them into usable planet metrics.
type task struct {
	enabled bool

	serverProcesses []Process
	upstreams       []Connections
	downstreams     []Connections
	mu              sync.Mutex
}

var singleton task

func init() {
	singleton = task{
		serverProcesses: []Process{},
		upstreams:       []Connections{},
		downstreams:     []Connections{},
		enabled:         false,
		mu:              sync.Mutex{},
	}
}

// InitTask initial states.
func InitTask(ctx context.Context, enabled bool) {
	singleton.enabled = enabled
}

// Process that binds on one or more network interfaces.
type Process struct {
	Name string // e.g. "node_exporter"
	Bind string // e.g. "0.0.0.0:9100"
	Port string // e.g. "9100"
}

// Connections socket connection metrics.
type Connections struct {
	LocalHostgroup  string
	LocalAddress    string
	RemoteHostgroup string
	RemoteAddress   string
	Port            string
	Protocol        string // tcp/udp
	ProcessName     string
}

// Get returns latest metrics from singleton.
func Get() ([]Process, []Connections, []Connections) {
	singleton.mu.Lock()
	up := singleton.upstreams
	down := singleton.downstreams
	serverProcesses := singleton.serverProcesses
	singleton.mu.Unlock()

	return serverProcesses, up, down
}

// Collect will collect fill singleton with latest data.
// nolint:cyclop
func Collect(ctx context.Context) error {
	if !singleton.enabled {
		return nil
	}

	startTime := time.Now()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Get server connection stat
	serverConnectionStat, err := network.ServerConnections(ctx)
	if err != nil {
		return fmt.Errorf("error getting server connections: %w", err)
	}
	serverProcesses, listeningPortsConns := parseProcessesAndListenPortsConns(serverConnectionStat)

	// Find current IP to replace loop-back address
	currentIP, err := network.LocalIP()
	if err != nil {
		return fmt.Errorf("error getting local IP address: %w", err)
	}

	// Upstreams and downstreams from every peered connection sockets (e.g. "ss -pant")
	var upstreams []Connections
	var downstreams []Connections

	includedConns := make(map[string]bool)
	for _, peeredConn := range serverConnectionStat.PeeredConnSockets {
		// Replace localhost or 127.0.0.1 with a more useful current address
		if peeredConn.LocalIP == "127.0.0.1" {
			peeredConn.LocalIP = currentIP.String()
		}

		// Find local Host inventory
		// This should be the same most of the time,
		// but we find LocalIP's inventory for every peeredConn in case there's interface address spoofing.
		localAddr, localHostgroup := getInventoryAddrAndHostgroup(peeredConn.LocalIP)

		// Find remote Host inventory
		remoteAddr, remoteHostgroup := getInventoryAddrAndHostgroup(peeredConn.RemoteIP)

		// Check whether this is a downstream/upstream connection tuple
		if listeningConn, foundListeningConn := listeningPortsConns[peeredConn.LocalPort]; foundListeningConn {
			// It's a downstream connection. The peerConn.localPort is one of the listening port.

			// Since it's a downstream conn, remote port is the listening server port
			remotePort := fmt.Sprint(peeredConn.LocalPort)

			// To track whether we have considered this connection
			connString := fmt.Sprintf("down_%s_%s_%v_%s", remoteHostgroup, remoteAddr, peeredConn.LocalPort, peeredConn.Protocol)
			// Prevents duplicate downstream conn entries
			if _, ok := includedConns[connString]; ok {
				continue
			}
			includedConns[connString] = true

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
				Port:            remotePort,
				Protocol:        peeredConn.Protocol,
				ProcessName:     peeredConn.ProcessName,
			})
		} else if remoteAddr != "localhost" {
			// It's an upstream connection otherwise.

			remotePort := fmt.Sprint(peeredConn.RemotePort)

			// To track whether we have considered this connection
			connString := fmt.Sprintf("up_%s_%s_%s_%s", remoteHostgroup, remoteAddr, remotePort, peeredConn.Protocol)
			// Prevents duplicate upstream conn entries
			if _, ok := includedConns[connString]; ok {
				continue
			}
			includedConns[connString] = true

			upstreams = append(upstreams, Connections{
				LocalHostgroup:  localHostgroup,
				RemoteHostgroup: remoteHostgroup,
				LocalAddress:    localAddr,
				RemoteAddress:   remoteAddr,
				Port:            remotePort,
				Protocol:        peeredConn.Protocol,
				ProcessName:     peeredConn.ProcessName,
			})
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

// parseProcessesAndListenPortsConns parses listening server processes and connections' ports that are in LISTEN state
// Listening server processes are used to know what processes may accept downstream connections.
// Listening connection ports are used to check whether the local port in a given connection tuple is ephemeral or is owned by a server process.
func parseProcessesAndListenPortsConns(serverConnectionStat network.ServerConnectionStat) ([]Process, map[uint32]network.ListeningConnSocket) {
	// Listening server processes
	processes := []Process{}

	// Listening server ports
	listeningPortsConns := make(map[uint32]network.ListeningConnSocket)

	// Iterate over connection sockets that are in LISTEN state
	for _, listeningConn := range serverConnectionStat.ListeningConnSockets {
		// Build serverProcesses from server LISTEN sockets
		processes = append(processes, Process{
			Name: listeningConn.ProcessName,
			Bind: fmt.Sprintf("%v:%v", listeningConn.LocalIP, listeningConn.LocalPort),
			Port: fmt.Sprint(listeningConn.LocalPort),
		})

		// Build list of listening server ports from server LISTEN sockets
		listeningPortsConns[listeningConn.LocalPort] = listeningConn
		log.Debugf("Server listening on: %v:%v [process:%v]", listeningConn.LocalIP, listeningConn.LocalPort, listeningConn.ProcessName)
	}

	return processes, listeningPortsConns
}

// getInventoryAddrAndHostgroup returns address/domain and hostgroup of the given IP based on inventory data.
func getInventoryAddrAndHostgroup(targetIP string) (string, string) {
	inventoryHosts := inventory.Get()

	var addr, hostgroup string
	if host, found := inventoryHosts.GetHost(targetIP); found {
		addr = host.Domain
		hostgroup = host.Hostgroup
	}
	if addr == "" {
		addr = targetIP
	}

	return addr, hostgroup
}
