package transfer

import (
	"fmt"

	"resource"
)

type holder struct {
	conn *resource.Conn
}

var global *resource.Conn

func returned() *resource.Conn {
	conn := resource.Dial()
	return conn
}

func returnedWithError() (*resource.Conn, error) {
	conn := resource.Dial()
	return conn, nil
}

func returnedSomethingElse() (*resource.Conn, error) {
	conn := resource.Dial() // want `\[dial\] Dial requires Close on conn before function exit`
	conn.Write()
	return nil, nil
}

func storedInField(h *holder) {
	h.conn = resource.Dial()
}

func storedInGlobal() {
	global = resource.Dial()
}

func storedInSlice(conns []*resource.Conn) {
	conns[0] = resource.Dial()
}

func storedInMap(conns map[string]*resource.Conn) {
	conns["first"] = resource.Dial()
}

func sent(conns chan *resource.Conn) {
	conns <- resource.Dial()
}

func passed() {
	conn := resource.Dial()
	finish(conn)
}

// passedInDefer hands the value to a helper that runs at exit: a deferred call
// is a call, and its argument takes the obligation along.
func passedInDefer() {
	conn := resource.Dial()
	defer finish(conn)
}

func finish(conn *resource.Conn) {
	_ = conn.Close()
}

func goroutine() {
	conn := resource.Dial()
	go serve(conn)
}

func serve(conn *resource.Conn) {
	conn.Write()
}

func closedInGoroutine() {
	conn := resource.Dial()
	go conn.Close()
}

func capturedByClosure() {
	conn := resource.Dial()
	callback := func() {
		conn.Write()
	}
	callback()
}

func variadic() {
	conn := resource.Dial()
	fmt.Println(conn)
}

func methodIsNotTransfer() {
	conn := resource.Dial() // want `\[dial\] Dial requires Close on conn before function exit`
	conn.Write()
}
