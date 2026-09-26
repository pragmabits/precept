// Package value does not type-check. Its test variant holds this file too, and
// user imports the package itself: the error is found in both.
package value

func Value() int { return "value" }
