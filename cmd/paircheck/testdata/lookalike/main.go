// Command example.test is a main package whose import path ends in .test, as
// the test main go test generates does: it is analyzed like any other.
package main

import "os"

func main() {
	file, err := os.Open("main.go")
	if err != nil {
		return
	}
	_ = file
}
