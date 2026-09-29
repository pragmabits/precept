// Package store is what the rule of the test names, relative to this module.
package store

type Store struct{}

func (s *Store) Begin() {}
func (s *Store) End()   {}
