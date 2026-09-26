// Package generic has generic functions, methods of generic types, and
// functions that take the value as a type parameter.
package generic

type Handle[T any] struct {
	value T
}

func Open[T any](value T) *Handle[T] { return &Handle[T]{value: value} }

func Close[T any](handle *Handle[T]) {}

type Box[T any] struct {
	value T
}

func NewBox[T any](value T) *Box[T] { return &Box[T]{value: value} }

func (b *Box[T]) Close() {}

type Cache[K comparable, V any] struct {
	values map[K]V
}

func NewCache[K comparable, V any]() *Cache[K, V] { return &Cache[K, V]{} }

func (c *Cache[K, V]) Close() {}

// Leased is what Hold and Free take: the L of one is a different type from the
// L of the other, so the rule names the slot on each side.
type Leased interface {
	Renew()
}

type Lease struct{}

func (l *Lease) Renew() {}

func Hold[L Leased](lease L) {}

func Free[L Leased](lease L) {}

// The value of these is built from type parameters: a slice of one, and a map
// of two. Crossed maps its one type parameter to itself, so it does not
// correspond with Unlock.
func HoldAll[L Leased](leases []L) {}

func FreeAll[L Leased](leases []L) {}

func Lock[K comparable, V any](entries map[K]V) {}

func Unlock[K comparable, V any](entries map[K]V) {}

func Crossed[K comparable](entries map[K]K) {}
