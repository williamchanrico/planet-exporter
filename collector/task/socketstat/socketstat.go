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

	// We use listening ports to determine whether a connection
	// is an ingress connection
	uListenPorts, peerConns, err := network.PeerConnections(ctx)
	if err != nil {
		return err
	}
	listeningPorts := make(map[string]bool)
	for _, p := range uListenPorts {
		listeningPorts[fmt.Sprint(p)] = true
	}

	inventoryHosts := inventory.Get()

	defaultLocalAddr, err := network.DefaultLocalAddr()
	if err != nil {
		return err
	}

	exists := make(map[string]bool)

	var upstreams []Metric
	var downstreams []Metric
	for _, conn := range peerConns {
		localPort := fmt.Sprint(conn.LocalPort)
		remotePort := fmt.Sprint(conn.RemotePort)

		if conn.LocalIP == "127.0.0.1" {
			conn.LocalIP = defaultLocalAddr.String()
		}
		if conn.RemoteIP == "127.0.0.1" {
			conn.RemoteIP = defaultLocalAddr.String()
		}

		var localAddr, localHostgroup, remoteAddr, remoteHostgroup string
		if localHostInfo, ok := inventoryHosts[conn.LocalIP]; ok {
			localAddr = localHostInfo.Domain
			localHostgroup = localHostInfo.Hostgroup

		}
		if localAddr == "" {
			localAddr = conn.LocalIP
		}
		if remoteHostInfo, ok := inventoryHosts[conn.RemoteIP]; ok {
			remoteAddr = remoteHostInfo.Domain
			remoteHostgroup = remoteHostInfo.Hostgroup

		}
		if remoteAddr == "" {
			remoteAddr = conn.RemoteIP
		}

		if listeningPorts[localPort] {
			existenceKey := fmt.Sprintf("down_%s_%s_%s_%s", remoteHostgroup, remoteAddr, localPort, conn.Protocol)

			if _, ok := exists[existenceKey]; !ok {
				downstreams = append(downstreams, Metric{
					LocalHostgroup:  localHostgroup,
					RemoteHostgroup: remoteHostgroup,
					LocalAddress:    localAddr,
					RemoteAddress:   remoteAddr,
					Port:            localPort,
					Protocol:        conn.Protocol,
				})
				exists[existenceKey] = true
			}
		} else {
			existenceKey := fmt.Sprintf("up_%s_%s_%s_%s", remoteHostgroup, remoteAddr, remotePort, conn.Protocol)

			if _, ok := exists[existenceKey]; !ok {
				upstreams = append(upstreams, Metric{
					LocalHostgroup:  localHostgroup,
					RemoteHostgroup: remoteHostgroup,
					LocalAddress:    localAddr,
					RemoteAddress:   remoteAddr,
					Port:            remotePort,
					Protocol:        conn.Protocol,
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
