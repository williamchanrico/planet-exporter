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

package server

import (
	"context"
	"net/http"
	"time"

	reuse "github.com/libp2p/go-reuseport"
)

// Server struct
type Server struct {
	server  *http.Server
	handler http.Handler
}

// New returns a new HTTP server
func New(handler http.Handler) *Server {
	return &Server{
		handler: handler,
	}
}

// Serve runs server
func (s *Server) Serve(addr string) error {
	s.server = &http.Server{
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		Handler:      s.handler,
	}

	listener, err := reuse.Listen("tcp4", addr)
	if err != nil {
		return err
	}

	return s.server.Serve(listener)
}

// Shutdown server
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
