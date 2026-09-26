package funcvalue

import "resource"

// A satisfier called through a method value.

func closedByMethodValue() {
	conn := resource.Dial()
	closer := conn.Close
	defer closer()
}

func methodValueOfAnother() {
	conn := resource.Dial() // want `\[dial\] Dial requires Close on conn before function exit`
	other := resource.Dial()
	closer := other.Close
	defer closer()
	_ = conn
}

func methodValueNeverCalled() {
	conn := resource.Dial() // want `\[dial\] Dial requires Close on conn before function exit`
	closer := conn.Close
	_ = closer
}

// A value that is a function the rule has to see called.

func released(s *resource.Semaphore) {
	release := s.Acquire()
	defer release()
}

func releasedInClosure(s *resource.Semaphore) {
	release := s.Acquire()
	defer func() {
		release()
	}()
}

func notReleased(s *resource.Semaphore) {
	release := s.Acquire() // want `\[acquire\] Acquire requires a call on release before function exit`
	_ = release
}

func callbackOfAnotherType(s *resource.Semaphore, callback func(int)) {
	release := s.Acquire() // want `\[acquire\] Acquire requires a call on release before function exit`
	callback(1)
	_ = release
}

func handedToCaller(s *resource.Semaphore) func() {
	return s.Acquire()
}

// A method value of a satisfier stands for the value: where it goes, the
// obligation goes.

type closers struct {
	close func() error
}

func methodValueStored(h *closers) {
	conn := resource.Dial()
	h.close = conn.Close
}

func methodValueReturned() func() error {
	conn := resource.Dial()
	return conn.Close
}

func methodValuePassed() {
	conn := resource.Dial()
	register(conn.Close)
}

func register(close func() error) {}
