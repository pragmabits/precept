package paircheck_test

import (
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/pragmabits/precept/paircheck"
)

func TestFlow(t *testing.T) {
	analyzer := build(t,
		rule("resource", "(*resource.Resource).Open", "(*resource.Resource).Close"),
		rule(
			"transaction",
			"(*resource.Resource).Begin",
			"(*resource.Resource).Commit",
			"(*resource.Resource).Rollback",
		),
	)
	analysistest.Run(t, analysistest.TestData(), analyzer, "flow")
}

func TestMatching(t *testing.T) {
	analyzer := build(t,
		rule("resource", "(*resource.Resource).Open", "(*resource.Resource).Close"),
		rule("value", "(resource.Value).Open", "(*resource.Value).Close"),
		rule("pool", "(*resource.Pool[T]).Acquire", "(*resource.Pool).Release"),
		rule("opener", "(resource.Opener).Open", "(resource.Opener).Close"),
	)
	analysistest.Run(t, analysistest.TestData(), analyzer, "matching")
}

func TestShapes(t *testing.T) {
	swap := rule("swap", "(*resource.Cache).Swap", "(*resource.Cache).Put")
	swap.Trigger.Slot = "result 0"
	analyzer := build(t,
		rule(
			"transaction",
			"(*resource.DB).Begin",
			"(*resource.Tx).Commit",
			"(*resource.Tx).Rollback",
		),
		rule("acquire", "resource.Acquire", "resource.Release"),
		rule("handle", "resource.OpenHandle", "resource.CloseHandle"),
		rule("dial", "resource.Dial", "(*resource.Conn).Close"),
		swap,
	)
	analysistest.Run(t, analysistest.TestData(), analyzer, "shapes")
}

func TestAmbiguousSlotIsSilent(t *testing.T) {
	analyzer := build(t,
		rule("swap", "(*resource.Cache).Swap", "(*resource.Cache).Put"),
		rule("dial", "resource.Dial", "(*resource.Conn).Close"),
	)
	analysistest.Run(t, analysistest.TestData(), analyzer, "ambiguous")
}

func TestInvisibleSatisfier(t *testing.T) {
	analyzer := build(t,
		rule("finish", "resource.Dial", "other.Finish"),
		rule("recycle", "(*resource.Cache).Swap", "other.Recycle"),
	)
	analysistest.Run(t, analysistest.TestData(), analyzer, "invisible")
}

func TestFailure(t *testing.T) {
	analyzer := build(t,
		rule("start", "(*resource.Resource).Start", "(*resource.Resource).Stop"),
		rule(
			"transaction",
			"(*resource.DB).Begin",
			"(*resource.Tx).Commit",
			"(*resource.Tx).Rollback",
		),
	)
	analysistest.Run(t, analysistest.TestData(), analyzer, "failure")
}

func TestOpenOnError(t *testing.T) {
	start := rule("start", "(*resource.Resource).Start", "(*resource.Resource).Stop")
	start.OpenOnError = true
	analysistest.Run(t, analysistest.TestData(), build(t, start), "failurestrict")
}

func TestIdentity(t *testing.T) {
	analyzer := build(t,
		rule("resource", "(*resource.Resource).Open", "(*resource.Resource).Close"),
		rule("dial", "resource.Dial", "(*resource.Conn).Close"),
	)
	analysistest.Run(t, analysistest.TestData(), analyzer, "identity")
}

func TestDeferredClosure(t *testing.T) {
	coverages := map[string]paircheck.Coverage{
		"closureany":   "",
		"closureevery": paircheck.CoverageEveryPath,
		"closurenone":  paircheck.CoverageNone,
	}
	for pattern, coverage := range coverages {
		t.Run(pattern, func(t *testing.T) {
			resource := rule("resource", "(*resource.Resource).Open", "(*resource.Resource).Close")
			resource.DeferredClosure = coverage
			transaction := rule(
				"transaction",
				"(*resource.DB).Begin",
				"(*resource.Tx).Commit",
				"(*resource.Tx).Rollback",
			)
			transaction.DeferredClosure = coverage
			analysistest.Run(t, analysistest.TestData(), build(t, resource, transaction), pattern)
		})
	}
}

func TestExits(t *testing.T) {
	analyzer := build(t, rule("resource", "(*resource.Resource).Open", "(*resource.Resource).Close"))
	analysistest.Run(t, analysistest.TestData(), analyzer, "exits")
}

func TestDispatch(t *testing.T) {
	analyzer := build(t, rule("dial", "resource.Dial", "(*resource.Conn).Close"))
	analysistest.Run(t, analysistest.TestData(), analyzer, "dispatch")
}

func TestTransfer(t *testing.T) {
	disabled := false
	partial := rule("dial", "resource.Dial", "(*resource.Conn).Close")
	partial.Transfer = paircheck.Transfer{Argument: &disabled}
	none := rule("dial", "resource.Dial", "(*resource.Conn).Close")
	none.Transfer = paircheck.Transfer{Return: &disabled, Store: &disabled, Argument: &disabled}
	rules := map[string]paircheck.Rule{
		"transfer":        rule("dial", "resource.Dial", "(*resource.Conn).Close"),
		"transferpartial": partial,
		"transfernone":    none,
	}
	for pattern, current := range rules {
		t.Run(pattern, func(t *testing.T) {
			analysistest.Run(t, analysistest.TestData(), build(t, current), pattern)
		})
	}
}

func build(t *testing.T, rules ...paircheck.Rule) *analysis.Analyzer {
	t.Helper()
	analyzer, err := paircheck.New(paircheck.Config{Rules: rules})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return analyzer
}

func rule(id, trigger string, satisfiers ...string) paircheck.Rule {
	calls := make([]paircheck.Call, 0, len(satisfiers))
	for _, satisfier := range satisfiers {
		calls = append(calls, paircheck.Call{Name: satisfier})
	}
	return paircheck.Rule{
		ID:         id,
		Trigger:    paircheck.Call{Name: trigger},
		Satisfiers: calls,
	}
}
