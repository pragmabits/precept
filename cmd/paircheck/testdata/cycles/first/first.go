// Package first imports second in its test, and second imports first: the
// test has an import cycle, which go list reports with no position, as it does
// the one of third and fourth.
package first
