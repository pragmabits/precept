// Package labels declares labels, which break, continue and goto name.
package labels

func first(rows [][]int) int {
	found := -1
cfgRows: // want `"cfgRows" is forbidden by pattern "cfg" for label$`
	for _, row := range rows {
		for _, value := range row {
			if value == 0 {
				continue cfgRows
			}
			if value < 0 {
				found = value
				break cfgRows
			}
		}
	}
	return found
}

func countdown(count int) int {
	retryCount := count // want `"retryCount" is forbidden by pattern "\^retry" for local-var$`
retryAgain:
	if retryCount > 0 {
		retryCount--
		goto retryAgain
	}
	return retryCount
}

// nested declares a label again in a function literal, where the labels of
// the function around it are out of scope.
func nested() {
cfgOuter: // want `"cfgOuter" is forbidden by pattern "cfg" for label$`
	for {
		inner := func() {
		cfgOuter: // want `"cfgOuter" is forbidden by pattern "cfg" for label$`
			for {
				break cfgOuter
			}
		}
		inner()
		break cfgOuter
	}
}
