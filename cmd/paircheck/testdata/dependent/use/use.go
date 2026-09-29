package use

import (
	"example.com/dependent/store"
	library "example.com/library/store"
)

func Use(own *store.Store, theirs *library.Store) {
	own.Begin()
	defer own.End()
	theirs.Begin(&library.Other{})
}
