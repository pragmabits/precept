package files_test

import (
	"os"
	"testing"
)

// TestOutside leaves the file open in the external test package.
func TestOutside(t *testing.T) {
	file, err := os.Open("files.go") // want `\[file\] Open requires Close on file before function exit`
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Stat(); err != nil {
		t.Error(err)
	}
}

// TestOutsideDeferred closes the file in a defer.
func TestOutsideDeferred(t *testing.T) {
	file, err := os.Open("files.go")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := file.Stat(); err != nil {
		t.Error(err)
	}
}
