// Package helpers does things with a connection it receives, for another
// package to call.
package helpers

import "resource"

var kept *resource.Conn

// CloseQuietly closes the connection on every path.
func CloseQuietly(conn *resource.Conn) { _ = conn.Close() }

// Describe uses the connection, and neither closes it nor lets it leave.
func Describe(conn *resource.Conn) string {
	conn.Write()
	return "conn"
}

// Keep stores the connection where it outlives the call.
func Keep(conn *resource.Conn) { kept = conn }

// CloseSometimes closes the connection on one path only.
func CloseSometimes(conn *resource.Conn, now bool) {
	if now {
		_ = conn.Close()
	}
}

// CloseThroughAnother closes the connection through another helper.
func CloseThroughAnother(conn *resource.Conn) { CloseQuietly(conn) }
