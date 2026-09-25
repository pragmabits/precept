package resource

import (
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
