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
	"fmt"
	"net/http"
	"time"

	reuse "github.com/libp2p/go-reuseport"
)

// Server struct.
type Server struct {
	server  *http.Server
	handler http.Handler
}

// New returns a new HTTP server.
func New(handler http.Handler) *Server {
	const (
		readTimeoutSeconds  = 15
		writeTimeoutSeconds = 15
	)

	return &Server{
		server: &http.Server{ // nolint:exhaustivestruct
			ReadTimeout:  readTimeoutSeconds * time.Second,
			WriteTimeout: writeTimeoutSeconds * time.Second,
			Handler:      handler,
		},
		handler: handler,
	}
}

// Serve runs server.
func (s *Server) Serve(addr string) error {
	listener, err := reuse.Listen("tcp4", addr)
	if err != nil {
		return fmt.Errorf("error creating server listener: %w", err)
	}

	if err = s.server.Serve(listener); err != nil {
		return fmt.Errorf("error on server serve: %w", err)
	}

	return nil
}

// Shutdown server.
func (s *Server) Shutdown(ctx context.Context) error {
	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("error on server shutdown: %w", err)
	}

	return nil
}
