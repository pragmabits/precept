// Package outside declares names of no kind a rule applies to: a renamed
// import and a label.
package outside

import cfgexternal "external"

func run() int {
cfgLoop:
	for range 3 {
		break cfgLoop
	}
	return cfgexternal.Cfg()
}
