// Package left and package right import each other outside their tests, and
// both have tests: the cycle is in the packages, and their test variants only
// inherit it.
package left

import _ "example.com/circular/right"
