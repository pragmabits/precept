package files

import (
	"os"
	"testing"
)

// TestStat leaves the file open: a test file is checked as the package's other
// files are. The test variant of the package holds files.go too, and each
// diagnostic of files.go is still expected once.
func TestStat(t *testing.T) {
	file, err := os.Open("files.go") // want `\[file\] Open requires Close on file before function exit`
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Stat(); err != nil {
		t.Error(err)
	}
}

// TestDeferred closes the file in a defer.
func TestDeferred(t *testing.T) {
	file, err := os.Open("files.go")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := file.Stat(); err != nil {
		t.Error(err)
	}
}
