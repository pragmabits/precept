package other

import "resource"

type Resource struct{}

func New() *Resource { return &Resource{} }

func (r *Resource) Open()  {}
func (r *Resource) Close() {}

func Finish(conn *resource.Conn)  {}
func Recycle(conn *resource.Conn) {}
