package names

import "testing"

func TestNewManager(t *testing.T) { // want `"TestNewManager" is forbidden by pattern "\(\?i\)manager\$" for function$`
	cfg := NewManager("test").cfg // want `"cfg" is forbidden: avoid the cfg abbreviation$`
	if cfg != "test" {
		t.Fail()
	}
}
