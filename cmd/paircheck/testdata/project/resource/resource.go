package resource

type Resource struct{}

func (r *Resource) Open()  {}
func (r *Resource) Close() {}

func Open() (*Resource, error) { return &Resource{}, nil }

type Conn struct{}

type Cache struct{}

func (c *Cache) Swap(old *Conn) *Conn { return old }
func (c *Cache) Put(conn *Conn)       {}

func Wrap(err error) error { return err }

type Opener interface {
	Open()
	Close()
}

type Pool interface{ Acquire(conn *Conn) }

type Recycler interface {
	Release(conn *Conn)
	Reset()
}

// SQLPool is a Pool, and has a Release of its own without being a Recycler.
type SQLPool struct{}

func (p *SQLPool) Acquire(conn *Conn) {}
func (p *SQLPool) Release(conn *Conn) {}
