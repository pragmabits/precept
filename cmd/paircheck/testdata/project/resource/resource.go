package resource

type Resource struct{}

func (r *Resource) Open()  {}
func (r *Resource) Close() {}

func Open() (*Resource, error) { return &Resource{}, nil }

type Conn struct{}

type Cache struct{}

func (c *Cache) Swap(old *Conn) *Conn { return old }
func (c *Cache) Put(conn *Conn)       {}
