package dispatch

import (
	"io"

	"resource"
)

func closedThroughInterface() {
	conn := resource.Dial()
	var closer io.Closer = conn
	defer closer.Close()
}

func closedThroughMergedInterface(other io.ReadCloser, fresh bool) {
	var source io.Closer = other
	if fresh {
		conn := resource.Dial()
		source = conn
	}
	defer source.Close()
}

func interfaceOfAnotherValue() {
	first := resource.Dial() // want `\[dial\] Dial requires Close on first before function exit`
	second := resource.Dial()
	var closer io.Closer = second
	closer.Close()
	_ = first
}

func anotherMethod() {
	conn := resource.Dial() // want `\[dial\] Dial requires Close on conn before function exit`
	var writer interface{ Write() } = conn
	writer.Write()
}
