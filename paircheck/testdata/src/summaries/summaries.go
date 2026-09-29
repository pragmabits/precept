package summaries

import (
	"helpers"
	"resource"
)

func closeHere(conn *resource.Conn) { _ = conn.Close() }

func describeHere(conn *resource.Conn) { conn.Write() }

// closedByHelper hands the connection to a helper that closes it.
func closedByHelper() {
	conn := resource.Dial()
	helpers.CloseQuietly(conn)
}

// closedByDeferredHelper defers a helper that closes the connection.
func closedByDeferredHelper() {
	conn := resource.Dial()
	defer helpers.CloseQuietly(conn)
}

// closedThroughTwoHelpers hands the connection to a helper that closes it
// through another.
func closedThroughTwoHelpers() {
	conn := resource.Dial()
	helpers.CloseThroughAnother(conn)
}

// closedByLocalHelper hands the connection to a helper of its own package.
func closedByLocalHelper() {
	conn := resource.Dial()
	closeHere(conn)
}

// describedAndLeaked hands the connection to a helper that only uses it.
func describedAndLeaked() {
	conn := resource.Dial() // want `\[dial\] Dial requires Close on conn before function exit`
	_ = helpers.Describe(conn)
}

// describedLocallyAndLeaked hands the connection to a helper of its own
// package that only uses it.
func describedLocallyAndLeaked() {
	conn := resource.Dial() // want `\[dial\] Dial requires Close on conn before function exit`
	describeHere(conn)
}

// describedAndClosed uses the connection through a helper, then closes it.
func describedAndClosed() {
	conn := resource.Dial()
	_ = helpers.Describe(conn)
	_ = conn.Close()
}

// kept hands the connection to a helper that stores it: it leaves with it.
func kept() {
	conn := resource.Dial()
	helpers.Keep(conn)
}

// closedSometimes hands the connection to a helper that closes it on one path.
func closedSometimes() {
	conn := resource.Dial()
	helpers.CloseSometimes(conn, true)
}
