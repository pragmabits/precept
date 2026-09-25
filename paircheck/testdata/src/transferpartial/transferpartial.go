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

func passed() {
	conn := resource.Dial() // want `\[dial\] Dial requires Close on conn before function exit`
	finish(conn)
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
