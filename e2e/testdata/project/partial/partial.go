// Package partial sees pair.Both but not release.Left: the rule pair cannot
// settle its slot here, and is skipped without a diagnostic.
package partial

import "example.com/project/pair"

func Unseen() {
	left, right := pair.Both()
	_, _ = left, right
}
