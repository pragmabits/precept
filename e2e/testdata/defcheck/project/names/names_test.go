package names

import "testing"

// TestNewManager is left out: go test runs it, and its name is not the
// code's to choose.
func TestNewManager(t *testing.T) {
	cfg := NewManager("test").cfg // want `"cfg" is forbidden: avoid the cfg abbreviation$`
	if cfg != "test" {
		t.Fail()
	}
}
