// Package sessions uses the stores under the rule session, which lists memory
// and disk among its implementations, disk with an id of its own.
package sessions

import (
	"example.com/project/session"
	"example.com/project/session/disk"
	"example.com/project/session/memory"
)

// ThroughInterface opens through the interface and never closes.
func ThroughInterface(s session.Store) {
	s.Open() // want `\[session\] Open requires Close on s before function exit`
}

// Memory opens a memory store and never closes it.
func Memory() {
	s := memory.New()
	s.Open() // want `\[session\] Open requires Close on s before function exit`
}

// MemoryClosed opens a memory store and closes it.
func MemoryClosed() {
	s := memory.New()
	s.Open()
	defer s.Close()
}

// Disk opens a disk store and never closes it: its findings carry its own id.
func Disk() {
	s := disk.New()
	s.Open() // want `\[session-disk\] Open requires Close on s before function exit`
}
