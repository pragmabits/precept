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

func Count() int {
	manager := cacheManager
	service := NewManager("count")
	return len(manager) + service.Manager + int(mgrLimit)
}
