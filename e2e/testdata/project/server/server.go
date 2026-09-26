// Package server serves idempotently: Serve on a running server does nothing,
// and one Shutdown ends it.
package server

type Server struct{}

func (s *Server) Serve() {}

func (s *Server) Shutdown() {}
