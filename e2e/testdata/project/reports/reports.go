// Package reports hands the files it opens to fileutil, under the rule file,
// whose transfer is the default: a helper that closes the file discharges it,
// and one that only uses it leaves it open.
package reports

import (
	"os"

	"example.com/project/fileutil"
)

// Closed hands the file to a helper that closes it.
func Closed(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer fileutil.CloseQuietly(f)
	return nil
}

// Described hands the file to a helper that only reads its name.
func Described(path string) (string, error) {
	f, err := os.Open(path) // want `\[file\] Open requires Close on f before function exit`
	if err != nil {
		return "", err
	}
	return fileutil.Describe(f), nil
}
