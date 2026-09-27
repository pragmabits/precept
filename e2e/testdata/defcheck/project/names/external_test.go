package names_test

import (
	"testing"

	"example.com/names/names"
)

func TestCount(t *testing.T) {
	mgrCount := names.Count() // want `"mgrCount" is forbidden by pattern "\^mgr" for local-var$`
	if mgrCount == 0 {
		t.Fail()
	}
}
