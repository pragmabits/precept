package implementations

import (
	"handle"
	"resource"
)

// throughInterface opens through the interface and never closes.
func throughInterface(o resource.Opener) {
	o.Open() // want `\[opener\] Open requires Close on o before function exit`
}

// fileClosed opens a file of the package of the interface and closes it.
func fileClosed() {
	f := &resource.File{}
	f.Open()
	defer f.Close()
}

// fileLeaks opens a file and never closes it.
func fileLeaks() {
	f := &resource.File{}
	f.Open() // want `\[opener\] Open requires Close on f before function exit`
}

// handleLeaks opens a handle, whose findings carry its own id.
func handleLeaks() {
	h := handle.New()
	h.Open() // want `\[handle\] Open requires Close on h before function exit`
}

// handleClosedThroughInterface closes a handle through the interface.
func handleClosedThroughInterface() {
	h := handle.New()
	h.Open()
	var o resource.Opener = h
	defer o.Close()
}

// resourceLeaks opens a type the rule does not list.
func resourceLeaks() {
	r := resource.New()
	r.Open()
}
