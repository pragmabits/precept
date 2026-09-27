package names

import "testing"

func TestLoad(t *testing.T) {
	cfg := Load()
	if cfg == 0 {
		t.Fail()
	}
}
