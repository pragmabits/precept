// Package servers uses server under serve, which counts every Serve, and
// serve-idempotent, which counts one.
package servers

import "example.com/project/server"

// ServedTwice serves twice and shuts down once.
func ServedTwice(s *server.Server) {
	s.Serve() // want `\[serve\] Serve requires Shutdown on s before function exit`
	s.Serve()
	defer s.Shutdown()
}

func ServedOnce(s *server.Server) {
	s.Serve()
	defer s.Shutdown()
}

// NeverShut never shuts down: idempotent or not, the server stays up.
func NeverShut(s *server.Server) {
	s.Serve() // want `\[serve\] Serve requires Shutdown on s before function exit` `\[serve-idempotent\] Serve requires Shutdown on s before function exit`
	s.Serve() // want `\[serve\] Serve requires Shutdown on s before function exit` `\[serve-idempotent\] Serve requires Shutdown on s before function exit`
}
