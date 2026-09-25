package transfernone

import "resource"

type holder struct {
	conn *resource.Conn
}

func returned() *resource.Conn {
	conn := resource.Dial() // want `\[dial\] Dial requires Close on conn before function exit`
	return conn
}

func stored(h *holder) {
	h.conn = resource.Dial() // want `\[dial\] Dial requires Close on h.conn before function exit`
}
