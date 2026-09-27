package leak

import "os"

func Plain() {
	file, err := os.Open("plain")
	if err != nil {
		return
	}
	_ = file.Name()
}
