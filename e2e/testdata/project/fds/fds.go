// Package fds uses fdio under fd: trigger slot result 0, transfer narrowed so
// that reading through the descriptor does not take the obligation along.
package fds

import "example.com/project/fdio"

// ReadLeak returns on a read error with the descriptor open.
func ReadLeak(path string) error {
	fd, err := fdio.Open(path, 0, 0) // want `\[fd\] Open requires Close on fd before function exit`
	if err != nil {
		return err
	}
	var buffer [1]byte
	if _, err := fdio.Read(fd, buffer[:]); err != nil {
		return err
	}
	return fdio.Close(fd)
}

// ReadClosed closes the descriptor in a defer.
func ReadClosed(path string) error {
	fd, err := fdio.Open(path, 0, 0)
	if err != nil {
		return err
	}
	defer fdio.Close(fd)
	var buffer [1]byte
	_, err = fdio.Read(fd, buffer[:])
	return err
}
