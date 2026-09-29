package storedvalue

import "resource"

type holder struct {
	close func() error
	other func() error
	conn  *resource.Conn
}

// rebind keeps in close the method value of the connection the holder keeps.
func (h *holder) rebind() { h.close = h.conn.Close }

// fromField calls the method value it kept in a field.
func fromField(h *holder) {
	conn := resource.Dial()
	h.close = conn.Close
	defer h.close()
}

// fromMap calls the method value it kept in a map, by the same key.
func fromMap(closers map[string]func() error) {
	conn := resource.Dial()
	closers["conn"] = conn.Close
	defer closers["conn"]()
}

// otherField calls what another field of the same holder keeps.
func otherField(h *holder) {
	conn := resource.Dial() // want `\[dial\] Dial requires Close on conn before function exit`
	h.close = conn.Close
	defer h.other()
}

// otherKey calls what another key of the map holds.
func otherKey(closers map[string]func() error) {
	conn := resource.Dial() // want `\[dial\] Dial requires Close on conn before function exit`
	closers["conn"] = conn.Close
	defer closers["other"]()
}

// otherConnection keeps the method value of another connection in the field.
func otherConnection(h *holder) {
	conn := resource.Dial() // want `\[dial\] Dial requires Close on conn before function exit`
	other := resource.Dial()
	h.close = other.Close
	defer h.close()
	_ = conn
}

// neverCalled keeps the method value and never calls it.
func neverCalled(h *holder) {
	conn := resource.Dial() // want `\[dial\] Dial requires Close on conn before function exit`
	h.close = conn.Close
}

// otherMapType keeps the method value in a map keyed by text, and calls from a
// map keyed by truth.
func otherMapType(byName map[string]func() error, byFlag map[bool]func() error) {
	conn := resource.Dial() // want `\[dial\] Dial requires Close on conn before function exit`
	byName["conn"] = conn.Close
	defer byFlag[true]()
}

// aliasing keeps the method value in one holder and calls through another,
// which may be the same one: only conn may be closed there.
func aliasing(g, h *holder) {
	conn := resource.Dial()
	other := resource.Dial() // want `\[dial\] Dial requires Close on other before function exit`
	g.close = conn.Close
	defer h.close()
	_ = other
}

// rebound keeps a method value, then calls what may store another where the
// deferred call reads it.
func rebound(h *holder) {
	conn := resource.Dial()
	spare := resource.Dial()
	h.conn = conn
	h.close = spare.Close
	h.rebind()
	defer h.close()
}

// twoMaps keeps method values by one key in two maps it made, and calls from
// the first.
func twoMaps() {
	first, second := map[string]func() error{}, map[string]func() error{}
	conn := resource.Dial()
	other := resource.Dial() // want `\[dial\] Dial requires Close on other before function exit`
	first["c"] = conn.Close
	second["c"] = other.Close
	defer first["c"]()
}

// fromArray calls the method value it kept in an element of an array.
func fromArray() {
	conn := resource.Dial()
	var closers [1]func() error
	closers[0] = conn.Close
	defer closers[0]()
}
