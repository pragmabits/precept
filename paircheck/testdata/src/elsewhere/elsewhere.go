// Package elsewhere uses a handle and does not see resource.Opener.
package elsewhere

import "handle"

func leaks() {
	h := handle.New()
	h.Open() // want `\[handle\] Open requires Close on h before function exit`
}

func closed() {
	h := handle.New()
	h.Open()
	defer h.Close()
}
