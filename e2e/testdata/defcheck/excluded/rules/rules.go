// Package rules declares names the rules of linters.exclusions drop, by the
// message of their finding or by the line they are declared on, and one they
// leave.
package rules

var cfgText = 1

var cfgSource = 2 // marked

var cfgLeft = 3 // want `^declaration name "cfgLeft" is forbidden by pattern "\^cfg" for package-var$`
