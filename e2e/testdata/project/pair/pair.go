// Package pair returns two values; release.Left discharges the first. Without
// release in sight, the rule pair cannot tell which of the two it is on.
package pair

type Left struct{}

type Right struct{}

func Both() (*Left, *Right) { return &Left{}, &Right{} }
