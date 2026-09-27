// Package blank declares _ wherever the language lets a declaration name it:
// no name is declared there.
package blank

var _ = 1

const _ = 2

type value struct {
	_ int
}

func (_ value) method(_ int) (_ int) {
	var _ = 3
	_, named := 4, 5 // want `"named" is forbidden by pattern "\^_\$\|\^named\$" for local-var$`
	return named
}
