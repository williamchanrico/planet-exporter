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

package network

import (
	"context"
	"net"
	"syscall"

	"planet-exporter/pkg/process"

	psutilnet "github.com/shirou/gopsutil/net"
)

type Peer struct {
	LocalIP     string
	LocalPort   uint32
	RemoteIP    string
	RemotePort  uint32
	Protocol    string
	ProcessName string
}

type Server struct {
	ProcessPid  int32
	ProcessName string
	Address     string
	Port        uint32
}

// ServerConnections returns LISTENING ports and peer connection tuples that are in ESTABLISHED or TIME_WAIT state
// Limited to 4096 connections per running process
func ServerConnections(ctx context.Context) ([]Server, []Peer, error) {
	processTable, err := process.GetProcessTable(ctx)
	if err != nil {
		return nil, nil, err
	}

	// "01": "ESTABLISHED",
	// "06": "TIME_WAIT",
	// "0A": "LISTEN",
	allConns, err := psutilnet.ConnectionsMaxWithContext(ctx, "all", 4096)
	if err != nil {
		return nil, nil, err
	}

	// Get listening sockets
	servers := []Server{}
	for _, conn := range allConns {
		if conn.Status == "LISTEN" {
			servers = append(servers, Server{
				ProcessName: processTable[int(conn.Pid)],
				ProcessPid:  conn.Pid,
				Address:     conn.Laddr.IP,
				Port:        conn.Laddr.Port,
			})
		}
	}

	// Get peer connection sockets
	peerTuples := []Peer{}
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
			processName := processTable[int(conn.Pid)]
			peerTuples = append(peerTuples, Peer{
				LocalIP:     conn.Laddr.IP,
				LocalPort:   conn.Laddr.Port,
				RemoteIP:    conn.Raddr.IP,
				RemotePort:  conn.Raddr.Port,
				Protocol:    proto,
				ProcessName: processName,
			})
		}
	}

	return servers, peerTuples, nil
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
