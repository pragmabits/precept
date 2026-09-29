// Package diskonly uses a disk store and does not see session.Store: the
// store's own methods stand for the interface's.
package diskonly

import "example.com/project/session/disk"

// Leak opens a disk store and never closes it.
func Leak() {
	s := disk.New()
	s.Open() // want `\[session-disk\] Open requires Close on s before function exit`
}
