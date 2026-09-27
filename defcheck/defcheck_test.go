package defcheck_test

import (
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/pragmabits/precept/defcheck"
)

func TestDeclarations(t *testing.T) {
	analyzer := build(t, defcheck.Rule{Pattern: "cfg"})
	analysistest.Run(t, analysistest.TestData(), analyzer, "declarations")
}

func TestUses(t *testing.T) {
	analyzer := build(t, defcheck.Rule{Pattern: "cfg"}, defcheck.Rule{Pattern: "Cfg"})
	analysistest.Run(t, analysistest.TestData(), analyzer, "uses")
}

func TestOutsideEveryKind(t *testing.T) {
	analyzer := build(t, defcheck.Rule{Pattern: "cfg"})
	analysistest.Run(t, analysistest.TestData(), analyzer, "outside")
}

func TestBlank(t *testing.T) {
	analyzer := build(t, defcheck.Rule{Pattern: "^_$|^named$"})
	analysistest.Run(t, analysistest.TestData(), analyzer, "blank")
}

func TestSelectedKinds(t *testing.T) {
	analyzer := build(t,
		defcheck.Rule{Pattern: "cfg", Kinds: []defcheck.Kind{defcheck.KindLocalVar}},
		defcheck.Rule{Pattern: "^nothing$"},
	)
	analysistest.Run(t, analysistest.TestData(), analyzer, "selected")
}

func TestLanguage(t *testing.T) {
	analyzer := build(t, defcheck.Rule{Pattern: "cfg"})
	analysistest.Run(t, analysistest.TestData(), analyzer, "language")
}

func TestMessage(t *testing.T) {
	analyzer := build(t,
		defcheck.Rule{Pattern: "^cfg$", Message: "avoid the cfg abbreviation"},
		defcheck.Rule{
			Pattern: "cfg",
			Kinds:   []defcheck.Kind{defcheck.KindLocalVar, defcheck.KindLocalVar},
		},
	)
	analysistest.Run(t, analysistest.TestData(), analyzer, "message")
}

func TestWithoutRules(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), build(t), "external")
}

// TestReportsInOrder reads the diagnostics in the order the analyzer reports
// them: by position, and at one position by the order of the rules.
func TestReportsInOrder(t *testing.T) {
	analyzer := build(
		t,
		defcheck.Rule{Pattern: "cfg"},
		defcheck.Rule{Pattern: "cfg", Message: "again"},
	)
	results := analysistest.Run(t, analysistest.TestData(), analyzer, "ordered")
	for _, result := range results {
		for index := 1; index < len(result.Diagnostics); index++ {
			previous, current := result.Diagnostics[index-1], result.Diagnostics[index]
			byRule := !strings.HasSuffix(previous.Message, "again") &&
				strings.HasSuffix(current.Message, "again")
			if previous.Pos > current.Pos || previous.Pos == current.Pos && !byRule {
				t.Errorf(
					"diagnostic %d, %q, comes after %q",
					index,
					current.Message,
					previous.Message,
				)
			}
		}
	}
}

func build(t *testing.T, rules ...defcheck.Rule) *analysis.Analyzer {
	t.Helper()
	analyzer, err := defcheck.New(defcheck.Config{Rules: rules})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return analyzer
}
