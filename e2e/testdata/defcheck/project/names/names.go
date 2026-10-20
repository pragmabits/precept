// Package names declares names the rules forbid, and uses them, which
// declares nothing.
package names

import "strings"

var cacheManager = map[string]int{} // want `^declaration name "cacheManager" is forbidden by pattern "\(\?i\)manager\$" for package-var$`

// TaskManager is a type: the manager rule leaves types out.
type TaskManager struct{}

// Service embeds TaskManager, a field with no name of its own.
type Service struct {
	cfg     string // want `^declaration name "cfg" is forbidden: avoid the cfg abbreviation$`
	Manager int    // want `^declaration name "Manager" is forbidden by pattern "\(\?i\)manager\$" for field$`
	TaskManager
}

func NewManager(cfg string) *Service { // want `"NewManager" is forbidden by pattern "\(\?i\)manager\$" for function$` `"cfg" is forbidden: avoid the cfg abbreviation$`
	return &Service{cfg: cfg}
}

func (s *Service) SessionManager() string { // want `"SessionManager" is forbidden by pattern "\(\?i\)manager\$" for method$`
	cfg := s.cfg // want `"cfg" is forbidden: avoid the cfg abbreviation$`
	cfg = strings.ToUpper(cfg)
	return cfg
}

type mgrState int // want `"mgrState" is forbidden by pattern "\^mgr" for type$`

const mgrLimit mgrState = 3 // want `"mgrLimit" is forbidden by pattern "\^mgr" for constant$`

// Negative is the first negative value of rows, or 0.
func Negative(rows [][]int) int {
	found := 0
mgrRows: // want `"mgrRows" is forbidden by pattern "\^mgr" for label$`
	for _, row := range rows {
		for _, value := range row {
			if value < 0 {
				found = value
				break mgrRows
			}
		}
	}
	return found
}

// Attempts counts the attempts of a retry, up to limit. The loop rule leaves
// labels out.
func Attempts(limit int) int {
	count := 0
retryLoop:
	for innerLoop := range limit { // want `"innerLoop" is forbidden by pattern "\(\?i\)loop\$" for local-var$`
		count++
		if innerLoop == 2 {
			break retryLoop
		}
	}
	return count
}

func Count() int {
	manager := cacheManager
	service := NewManager("count")
	return len(manager) + service.Manager + int(mgrLimit)
}
