// Package sem hands back the function that releases what Acquire took.
package sem

type Semaphore struct{}

func (s *Semaphore) Acquire() func() { return func() {} }
