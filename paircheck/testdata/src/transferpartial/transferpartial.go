package transferpartial

import (
	"fmt"

	"resource"
)

type holder struct {
	conn *resource.Conn
}

func returned() *resource.Conn {
	conn := resource.Dial()
	return conn
}

func stored(h *holder) {
	h.conn = resource.Dial()
}

// passed hands the value to a helper that closes it on every path, which is
// a satisfier.
func passed() {
	conn := resource.Dial()
	finish(conn)
}

func passedInDefer() {
	conn := resource.Dial()
	defer finish(conn)
}

func finish(conn *resource.Conn) {
	_ = conn.Close()
}

func variadic() {
	conn := resource.Dial() // want `\[dial\] Dial requires Close on conn before function exit`
	fmt.Println(conn)
}

func goroutine() {
	conn := resource.Dial() // want `\[dial\] Dial requires Close on conn before function exit`
	go conn.Close()
}
