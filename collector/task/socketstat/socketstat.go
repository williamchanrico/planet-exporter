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
	once      sync.Once
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
	servers, peers, err := network.ServerConnections(ctx)
	if err != nil {
		return err
	}
	listeningServerPorts := make(map[uint32]network.Server)
	for _, p := range servers {
		listeningServerPorts[p.Port] = p
		log.Debugf("Server listening on: %v:%v [process:%v]", p.Address, p.Port, p.ProcessName)
	}

	inventoryHosts := inventory.Get()

	defaultLocalAddr, err := network.DefaultLocalAddr()
	if err != nil {
		return err
	}

	exists := make(map[string]bool)

	// Build upstreams and downstreams from every peers
	var upstreams []Connections
	var downstreams []Connections
	for _, peerConn := range peers {
		inventoryHosts["127.0.0.1"] = inventory.Host{
			Domain:    "localhost",
			Hostgroup: "localhost",
			IPAddress: "127.0.0.1",
		}

		if peerConn.LocalIP == "127.0.0.1" {
			peerConn.LocalIP = defaultLocalAddr.String()
		}

		var localAddr, localHostgroup, remoteAddr, remoteHostgroup string
		if localHostInfo, ok := inventoryHosts[peerConn.LocalIP]; ok {
			localAddr = localHostInfo.Domain
			localHostgroup = localHostInfo.Hostgroup
		}
		if localAddr == "" {
			localAddr = peerConn.LocalIP
		}
		if remoteHostInfo, ok := inventoryHosts[peerConn.RemoteIP]; ok {
			remoteAddr = remoteHostInfo.Domain
			remoteHostgroup = remoteHostInfo.Hostgroup

		}
		if remoteAddr == "" {
			remoteAddr = peerConn.RemoteIP
		}

		// If peerConn.localPort is one of the listening port, it's a downstream connection
		if srv, isListening := listeningServerPorts[peerConn.LocalPort]; isListening {
			existenceKey := fmt.Sprintf("down_%s_%s_%v_%s", remoteHostgroup, remoteAddr, peerConn.LocalPort, peerConn.Protocol)

			// Prevents duplicate entries
			if _, ok := exists[existenceKey]; !ok {

				// Usually it's from TIME_WAIT socket states that don't have Pids stored
				// So we put whoever is holding that localPort instead
				if peerConn.ProcessName == "" {
					peerConn.ProcessName = srv.ProcessName
				}

				downstreams = append(downstreams, Connections{
					LocalHostgroup:  localHostgroup,
					RemoteHostgroup: remoteHostgroup,
					LocalAddress:    localAddr,
					RemoteAddress:   remoteAddr,
					Port:            fmt.Sprint(peerConn.LocalPort),
					Protocol:        peerConn.Protocol,
					ProcessName:     peerConn.ProcessName,
				})
				exists[existenceKey] = true
			}

		} else if remoteAddr != "localhost" {
			remotePort := fmt.Sprint(peerConn.RemotePort)
			existenceKey := fmt.Sprintf("up_%s_%s_%s_%s", remoteHostgroup, remoteAddr, remotePort, peerConn.Protocol)

			if _, ok := exists[existenceKey]; !ok {
				upstreams = append(upstreams, Connections{
					LocalHostgroup:  localHostgroup,
					RemoteHostgroup: remoteHostgroup,
					LocalAddress:    localAddr,
					RemoteAddress:   remoteAddr,
					Port:            remotePort,
					Protocol:        peerConn.Protocol,
					ProcessName:     peerConn.ProcessName,
				})
				exists[existenceKey] = true
			}
		}
	}

	// Build serverProcesses from server LISTEN sockets
	serverProcesses := []Process{}
	for _, v := range servers {
		serverProcesses = append(serverProcesses, Process{
			Name: v.ProcessName,
			Bind: fmt.Sprintf("%v:%v", v.Address, v.Port),
			Port: fmt.Sprint(v.Port),
		})
	}

	singleton.mu.Lock()
	singleton.serverProcesses = serverProcesses
	singleton.upstreams = upstreams
	singleton.downstreams = downstreams
	singleton.mu.Unlock()

	log.Debugf("tasksocketstat.Collect retrieved %v upstreams metrics", len(upstreams))
	log.Debugf("tasksocketstat.Collect retrieved %v downstreams metrics", len(downstreams))
	log.Debugf("tasksocketstat.Collect process took %v", time.Now().Sub(startTime))
	return nil
}
