// Package fdio has the signatures of syscall.Open, Read and Close on Linux:
// Open takes an int mode beside the int it returns, so a rule on it names the
// slot.
package fdio

func Open(path string, mode int, perm uint32) (fd int, err error) { return 3, nil }

func Read(fd int, p []byte) (n int, err error) { return 0, nil }

func Close(fd int) error { return nil }
