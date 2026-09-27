// Package selected declares names the rules match in kinds a rule leaves out.
package selected

var cfgPackage = 1

func run(cfgParameter int) int {
	cfgLocal := cfgParameter // want `"cfgLocal" is forbidden by pattern "cfg" for local-var$`
	return cfgLocal + cfgPackage
}
