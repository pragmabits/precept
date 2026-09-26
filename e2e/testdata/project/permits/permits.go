// Package permits uses sem under permit, whose satisfier is a call of the
// function Acquire returned.
package permits

import "example.com/project/sem"

func Released(s *sem.Semaphore) {
	release := s.Acquire()
	defer release()
}

func ReleasedInClosure(s *sem.Semaphore) {
	release := s.Acquire()
	defer func() {
		release()
	}()
}

// NotReleased returns on one path without releasing.
func NotReleased(s *sem.Semaphore, fail bool) {
	release := s.Acquire() // want `\[permit\] Acquire requires a call on release before function exit`
	if fail {
		return
	}
	release()
}

// HandedToCaller returns the release function: the caller releases.
func HandedToCaller(s *sem.Semaphore) func() {
	return s.Acquire()
}

// CallbackOfAnotherType calls a callback, which is not the release function.
func CallbackOfAnotherType(s *sem.Semaphore, next func(int)) {
	release := s.Acquire() // want `\[permit\] Acquire requires a call on release before function exit`
	next(1)
	_ = release
}
