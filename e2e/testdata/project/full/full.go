// Package full sees pair.Both and release.Left, which settles the slot.
package full

import (
	"example.com/project/pair"
	"example.com/project/release"
)

func Seen() {
	left, _ := pair.Both() // want `\[pair\] Both requires Left on left before function exit`
	_ = left
}

func Released() {
	left, _ := pair.Both()
	defer release.Left(left)
}
