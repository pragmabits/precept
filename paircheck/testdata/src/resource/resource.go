package resource

import (
	"context"
	"errors"
	"os"
)

var ErrBusy = errors.New("busy")

type Resource struct{}

func New() *Resource { return &Resource{} }

func (r *Resource) Open()     {}
func (r *Resource) Close()    {}
func (r *Resource) Use()      {}
func (r *Resource) Begin()    {}
func (r *Resource) Commit()   {}
func (r *Resource) Rollback() {}

type Opener interface {
	Open()
	Close()
}

type File struct{}

func (f *File) Open()  {}
func (f *File) Close() {}

type Value struct{}

func (Value) Open()  {}
func (Value) Close() {}

type Pool[T any] struct{}

func (p *Pool[T]) Acquire() {}
func (p *Pool[T]) Release() {}

func Acquire(r *Resource) {}
func Release(r *Resource) {}

type Handle int

func OpenHandle(path string) (Handle, error) { return 0, nil }
func CloseHandle(h Handle)                   {}

type DB struct{}
type Tx struct{}

func (d *DB) Begin() (*Tx, error) { return &Tx{}, nil }
func (t *Tx) Commit() error       { return nil }
func (t *Tx) Rollback() error     { return nil }

// Store and Session take a context on both sides, as most APIs written since
// Go 1.7 do.
type Store struct{}
type Session struct{}

func (s *Store) Begin(ctx context.Context) (*Session, error) { return &Session{}, nil }
func (s *Session) Commit(ctx context.Context) error          { return nil }
func (s *Session) Rollback(ctx context.Context) error        { return nil }

// Enter keeps a scope in the context it returns, which Leave ends: the value
// is the context itself.
func Enter(ctx context.Context) context.Context { return ctx }
func Leave(ctx context.Context)                 {}

// Hold and Free take the value as a type parameter: to go/types, the L of one
// is a different type from the L of the other.
type Leased interface{ Renew() }

type Lease struct{}

func (l *Lease) Renew() {}

func Hold[L Leased](lease L) {}
func Free[L Leased](lease L) {}

// The value of these is built from type parameters: a slice of one, and a map
// of two. Crossed maps its one type parameter to itself.
func HoldAll[L Leased](leases []L)                {}
func FreeAll[L Leased](leases []L)                {}
func Lock[K comparable, V any](entries map[K]V)   {}
func Unlock[K comparable, V any](entries map[K]V) {}
func Crossed[K comparable](entries map[K]K)       {}

// Server serves idempotently: Serve on a running server does nothing, and one
// Shutdown ends it.
type Server struct{}

func (s *Server) Serve()    {}
func (s *Server) Shutdown() {}

// Semaphore hands back the function that releases what Acquire took.
type Semaphore struct{}

func (s *Semaphore) Acquire() func() { return func() {} }

type Conn struct{}

func Dial() *Conn            { return &Conn{} }
func (c *Conn) Close() error { return nil }
func (c *Conn) Write()       {}

type Cache struct{}

func (c *Cache) Swap(old *Conn) *Conn { return old }
func (c *Cache) Put(conn *Conn)       {}

func (r *Resource) Start() error { return nil }
func (r *Resource) Stop()        {}

// Quit ends the process, as a helper of another package would.
func Quit() { os.Exit(3) }
