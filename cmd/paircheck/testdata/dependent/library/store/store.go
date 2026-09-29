// Package store sits at the path the rule names relative to the module, in a
// dependency, where the rule would not bind: Store and Other both link Begin
// to End.
package store

type Store struct{}

type Other struct{}

func (s *Store) Begin(other *Other)         {}
func (s *Store) End(other *Other, _ *Other) {}
