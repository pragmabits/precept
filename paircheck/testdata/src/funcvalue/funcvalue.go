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

// A function the trigger returned is not one the function received.

func callsParameter(s *resource.Semaphore, next func()) {
	release := s.Acquire() // want `\[acquire\] Acquire requires a call on release before function exit`
	defer next()
	_ = release
}

func callsParameterInClosure(s *resource.Semaphore, next func()) {
	release := s.Acquire() // want `\[acquire\] Acquire requires a call on release before function exit`
	defer func() {
		next()
	}()
	_ = release
}

// callsReassigned calls a variable that holds, by then, the function Acquire
// returned.
func callsReassigned(s *resource.Semaphore, next func()) {
	release := s.Acquire()
	defer func() {
		next()
	}()
	next = release
}

// closesParameter closes a connection it received, which may be the one Dial
// returned: only under the call satisfier is the trigger's result taken as new.
func closesParameter(other *resource.Conn) {
	conn := resource.Dial()
	defer other.Close()
	_ = conn
}
