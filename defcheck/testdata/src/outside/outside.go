// Package outside declares a name of no kind a rule applies to: a renamed
// import.
package outside

import cfgexternal "external"

func run() int {
	return cfgexternal.Cfg()
}
