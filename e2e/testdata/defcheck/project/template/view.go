// Package template declares a forbidden name after a line directive, which a
// generator writes to say the code below came from a template: the finding is
// printed in this file, where the code is, as golangci-lint prints it.
package template

//line view.tmpl:5
var mgrTemplate = 1 // want `"mgrTemplate" is forbidden by pattern "\^mgr" for package-var$`
