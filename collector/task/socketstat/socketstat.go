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
	"sync"
	"time"

	"planet-exporter/collector/task/inventory"
	"planet-exporter/pkg/network"

	log "github.com/sirupsen/logrus"
)

type task struct {
	enabled bool

	upstreams   []Metric
	downstreams []Metric
	mu          sync.Mutex
}

var once sync.Once
var singleton task

func init() {
	singleton = task{
		upstreams:   []Metric{},
		downstreams: []Metric{},
		enabled:     false,
		mu:          sync.Mutex{},
	}
}

func InitTask(ctx context.Context, enabled bool) {
	singleton.enabled = enabled
}

// Metric contains values needed for prometheus metrics
type Metric struct {
	LocalHostgroup  string
	LocalAddress    string
	RemoteHostgroup string
	RemoteAddress   string
	Port            string
	Protocol        string // tcp/udp
	ProcessName     string
}

// Get returns latest metrics in singleton
func Get() ([]Metric, []Metric) {
	singleton.mu.Lock()
	up := singleton.upstreams
	down := singleton.downstreams
	singleton.mu.Unlock()

	return up, down
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
		log.Debugf("Listening server ports: %v [process:%v]", p.Port, p.ProcessName)
	}

	inventoryHosts := inventory.Get()

	defaultLocalAddr, err := network.DefaultLocalAddr()
	if err != nil {
		return err
	}

	exists := make(map[string]bool)

	var upstreams []Metric
	var downstreams []Metric
	for _, peerConn := range peers {

		if peerConn.LocalIP == "127.0.0.1" {
			peerConn.LocalIP = defaultLocalAddr.String()
		}
		if peerConn.RemoteIP == "127.0.0.1" {
			peerConn.RemoteIP = defaultLocalAddr.String()
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

				downstreams = append(downstreams, Metric{
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
		} else {
			remotePort := fmt.Sprint(peerConn.RemotePort)
			existenceKey := fmt.Sprintf("up_%s_%s_%s_%s", remoteHostgroup, remoteAddr, remotePort, peerConn.Protocol)

			if _, ok := exists[existenceKey]; !ok {
				upstreams = append(upstreams, Metric{
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

	singleton.mu.Lock()
	singleton.upstreams = upstreams
	singleton.downstreams = downstreams
	singleton.mu.Unlock()

	log.Debugf("tasksocketstat.Collect retrieved %v upstreams metrics", len(upstreams))
	log.Debugf("tasksocketstat.Collect retrieved %v downstreams metrics", len(downstreams))
	log.Debugf("tasksocketstat.Collect process took %v", time.Now().Sub(startTime))
	return nil
}
