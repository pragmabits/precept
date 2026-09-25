package leak

import "example.com/project/resource"

func Leak(r *resource.Resource) {
	r.Open()
}
