package idempotent

import "resource"

func servedTwice(s *resource.Server) {
	s.Serve() // want `\[counted\] Serve requires Shutdown on s before function exit`
	s.Serve()
	defer s.Shutdown()
}

func servedOnce(s *resource.Server) {
	s.Serve()
	defer s.Shutdown()
}

func neverShut(s *resource.Server) {
	s.Serve() // want `\[counted\] Serve requires Shutdown on s before function exit` `\[idempotent\] Serve requires Shutdown on s before function exit`
	s.Serve() // want `\[counted\] Serve requires Shutdown on s before function exit` `\[idempotent\] Serve requires Shutdown on s before function exit`
}
