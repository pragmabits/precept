package defcheck

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// prohibition is a rule after validation: the pattern compiled, and the kinds
// it applies to normalized.
type prohibition struct {
	pattern *regexp.Regexp
	kinds   []Kind
	message string
}

// forbids reports whether a declaration of kind named name breaks the rule.
func (p prohibition) forbids(name string, kind Kind) bool {
	return slices.Contains(p.kinds, kind) && p.pattern.MatchString(name)
}

// diagnose is the message of the diagnostic at a declaration of kind named
// name. A message of the rule's own replaces the pattern and the kind, never
// the name.
func (p prohibition) diagnose(name string, kind Kind) string {
	if p.message != "" {
		return fmt.Sprintf("declaration name %q is forbidden: %s", name, p.message)
	}
	return fmt.Sprintf(
		"declaration name %q is forbidden by pattern %q for %s",
		name,
		p.pattern.String(),
		kind,
	)
}

// key is the same for two prohibitions only when they have the same pattern,
// kinds and message.
func (p prohibition) key() string {
	kinds := make([]string, 0, len(p.kinds))
	for _, kind := range p.kinds {
		kinds = append(kinds, string(kind))
	}
	return fmt.Sprintf("%q %q %q", p.pattern.String(), strings.Join(kinds, " "), p.message)
}
