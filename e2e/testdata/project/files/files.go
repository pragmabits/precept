// Package files uses os.Open under the README's file rule.
package files

import (
	"io"
	"log"
	"os"
)

// Leak returns on an error path with the file open.
func Leak(path string) error {
	file, err := os.Open(path) // want `\[file\] Open requires Close on file before function exit`
	if err != nil {
		return err
	}
	if _, err := file.Stat(); err != nil {
		return err
	}
	return file.Close()
}

// Deferred closes the file in a defer.
func Deferred(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Stat()
	return err
}

// ThroughInterface closes the file through an io.ReadCloser holding it.
func ThroughInterface(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	var closer io.ReadCloser = file
	defer closer.Close()
	return nil
}

// Returned hands the file to its caller.
func Returned(path string) (*os.File, error) {
	return os.Open(path)
}

// FatalOnStat ends the process on one path: nothing is left to close.
func FatalOnStat(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	if _, err := file.Stat(); err != nil {
		log.Fatal(err)
	}
	_ = file.Close()
}

// PanicOnStat panics on one path, which abandons it.
func PanicOnStat(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	if _, err := file.Stat(); err != nil {
		panic(err)
	}
	_ = file.Close()
}

// ClosedByMethodValue closes through a method value.
func ClosedByMethodValue(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	closer := file.Close
	defer closer()
	return nil
}

// MethodValueNeverCalled keeps a method value of Close and never calls it.
func MethodValueNeverCalled(path string) error {
	file, err := os.Open(path) // want `\[file\] Open requires Close on file before function exit`
	if err != nil {
		return err
	}
	closer := file.Close
	_ = closer
	return nil
}

type closers struct {
	close func() error
}

// MethodValueStored keeps a method value of Close in a field: the obligation
// goes with it.
func (c *closers) MethodValueStored(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	c.close = file.Close
	return nil
}
