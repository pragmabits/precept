// Package message declares a name two rules match: one with a message of its
// own, and one whose kinds repeat.
package message

func run() int {
	cfg := 1 // want `^declaration name "cfg" is forbidden: avoid the cfg abbreviation$` `^declaration name "cfg" is forbidden by pattern "cfg" for local-var$`
	return cfg
}
