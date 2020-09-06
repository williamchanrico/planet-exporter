package server

import (
	"context"
	"net"
	"net/http"
	"time"
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

	listener, err := net.Listen("tcp4", addr)
	if err != nil {
		return err
	}

	return s.server.Serve(listener)
}

// Shutdown server
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
