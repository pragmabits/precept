// Package value does not type-check, and user imports it: the error is found
// in both.
package value

func Value() int { return "value" }
