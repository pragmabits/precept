// Package disk is a session store, and does not import session.
package disk

type Store struct{}

func New() *Store { return &Store{} }

func (s *Store) Open()  {}
func (s *Store) Close() {}
