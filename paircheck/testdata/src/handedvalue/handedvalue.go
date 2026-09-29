package handedvalue

import "resource"

// handedToDeferredClosure hands the release function to the deferred closure
// as its argument, which calls it.
func handedToDeferredClosure(s *resource.Semaphore) {
	release := s.Acquire()
	defer func(done func()) {
		done()
	}(release)
}
