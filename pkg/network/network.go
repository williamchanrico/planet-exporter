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

package network

import (
	"context"
	"net"
	"syscall"

	psutilnet "github.com/shirou/gopsutil/net"
)

type ConnectionTuple struct {
	LocalIP    string
	LocalPort  uint32
	RemoteIP   string
	RemotePort uint32
	Protocol   string
}

// PeerConnections returns LISTENING ports and tuples that are either in ESTABLISHED or TIME_WAIT state
// Limited to 4096 connections per running process
func PeerConnections(ctx context.Context) ([]uint32, []ConnectionTuple, error) {
	// "01": "ESTABLISHED",
	// "06": "TIME_WAIT",
	// "0A": "LISTEN",
	allConns, err := psutilnet.ConnectionsMaxWithContext(ctx, "all", 4096)
	if err != nil {
		return nil, nil, err
	}

	listens := filterListeningPorts(ctx, allConns)

	peerTuples := []ConnectionTuple{}
	for _, conn := range allConns {
		proto := ""
		switch conn.Type {
		case syscall.SOCK_STREAM:
			proto = "tcp"
		case syscall.SOCK_DGRAM:
			proto = "udp"
		default:
			proto = ""
		}
		switch conn.Status {
		case "TIME_WAIT", "ESTABLISHED":
			peerTuples = append(peerTuples, ConnectionTuple{
				LocalIP:    conn.Laddr.IP,
				LocalPort:  conn.Laddr.Port,
				RemoteIP:   conn.Raddr.IP,
				RemotePort: conn.Raddr.Port,
				Protocol:   proto,
			})
		}
	}

	return listens, peerTuples, nil
}

// DefaultLocalAddr returns default local IP address
func DefaultLocalAddr() (net.IP, error) {
	conn, err := net.Dial("udp", "8.8.8.8:53")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP, nil
}

// listeningPorts returns local ports in LISTENING state
func filterListeningPorts(ctx context.Context, conns []psutilnet.ConnectionStat) []uint32 {
	listenPorts := []uint32{}
	for _, conn := range conns {
		if conn.Status == "LISTEN" {
			listenPorts = append(listenPorts, conn.Laddr.Port)
		}
	}

	return listenPorts
}
