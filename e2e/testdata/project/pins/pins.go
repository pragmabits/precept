// Package pins uses pin under the pin rule.
package pins

import "example.com/project/pin"

// Leak returns on one path with the object pinned.
func Leak(fail bool) {
	object := &pin.Object{}
	pin.Pin(object) // want `\[pin\] Pin requires Unpin on object before function exit`
	if fail {
		return
	}
	pin.Unpin(object)
}

// Deferred unpins in a defer.
func Deferred() {
	object := &pin.Object{}
	pin.Pin(object)
	defer pin.Unpin(object)
}
