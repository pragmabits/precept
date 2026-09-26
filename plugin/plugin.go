// Package plugin registers the analyzers of this module as golangci-lint
// module plugins. It holds the one init function of the module: a module
// plugin registers itself when golangci-lint imports it for that effect, and
// the analyzer packages, which golangci-lint imports natively, have none.
package plugin

import (
	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"

	"github.com/pragmabits/precept/paircheck"
)

func init() {
	register.Plugin("paircheck", newPaircheck)
}

// linter is an analyzer built from its plugin settings.
type linter struct {
	analyzers []*analysis.Analyzer
}

// newPaircheck builds paircheck from the settings golangci-lint hands over,
// and refuses them before any package is analyzed.
func newPaircheck(settings any) (register.LinterPlugin, error) {
	config, err := register.DecodeSettings[paircheck.Config](settings)
	if err != nil {
		return nil, err
	}
	analyzer, err := paircheck.New(config)
	if err != nil {
		return nil, err
	}
	return linter{analyzers: []*analysis.Analyzer{analyzer}}, nil
}

// BuildAnalyzers returns the analyzer built from the settings.
func (l linter) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	return l.analyzers, nil
}

// GetLoadMode asks for type information: a rule is bound to its functions
// through their types.
func (l linter) GetLoadMode() string {
	return register.LoadModeTypesInfo
}
