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

// PeeredConnSocket represents connection socket with a peer (sockets in ESTABLISHED and TIME_WAIT states)
type PeeredConnSocket struct {
	LocalIP     string
	LocalPort   uint32
	RemoteIP    string
	RemotePort  uint32
	Protocol    string
	ProcessName string
}

// ListeningConnSocket represents a connection socket from a listening server process (sockets in LISTEN state)
type ListeningConnSocket struct {
	LocalIP     string
	LocalPort   uint32
	ProcessPid  int32
	ProcessName string
}

// ServerConnectionStat represents a connection status, similar to netstat or "ss -pant" and "ss -pantl"
type ServerConnectionStat struct {
	PeeredConnSockets    []PeeredConnSocket
	ListeningConnSockets []ListeningConnSocket
}

// ServerConnections returns LISTENING ports and peer connection tuples that are in ESTABLISHED or TIME_WAIT state
// Limited to 4096 connections per running process
func ServerConnections(ctx context.Context) (ServerConnectionStat, error) {
	processTable, err := process.GetProcessTable(ctx)
	if err != nil {
		return ServerConnectionStat{}, err
	}

	// "01": "ESTABLISHED",
	// "06": "TIME_WAIT",
	// "0A": "LISTEN",
	allConns, err := psutilnet.ConnectionsMaxWithContext(ctx, "all", 4096)
	if err != nil {
		return ServerConnectionStat{}, err
	}

	// Listening connection sockets
	listeningConns := []ListeningConnSocket{}
	// Peered connection tuples
	peeredConns := []PeeredConnSocket{}

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
		case "LISTEN":
			listeningConns = append(listeningConns, ListeningConnSocket{
				LocalIP:     conn.Laddr.IP,
				LocalPort:   conn.Laddr.Port,
				ProcessName: processTable[int(conn.Pid)],
				ProcessPid:  conn.Pid,
			})

		case "TIME_WAIT", "ESTABLISHED":
			peeredConns = append(peeredConns, PeeredConnSocket{
				LocalIP:     conn.Laddr.IP,
				LocalPort:   conn.Laddr.Port,
				RemoteIP:    conn.Raddr.IP,
				RemotePort:  conn.Raddr.Port,
				Protocol:    proto,
				ProcessName: processTable[int(conn.Pid)],
			})
		}
	}

	return ServerConnectionStat{
		PeeredConnSockets:    peeredConns,
		ListeningConnSockets: listeningConns,
	}, nil
}

// LocalIP returns default local IP address
// Note the "udp" protocol. The net.Dial() call won't actually establish any connection.
func LocalIP() (net.IP, error) {
	conn, err := net.Dial("udp", "8.8.8.8:53")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP, nil
}
