package paircheck_test

import (
	"fmt"
	"path/filepath"
	"strings"
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

func TestUnboundRuleFails(t *testing.T) {
	tests := []struct {
		defect string
		rule   paircheck.Rule
		want   string
	}{
		{
			defect: "ambiguous slot",
			rule:   rule("swap", "(*resource.Cache).Swap", "(*resource.Cache).Put"),
			want:   `rule "swap": ` + paircheck.ErrAmbiguousSlot.Error(),
		},
		{
			defect: "only a context in common",
			rule:   rule("scope", "resource.Enter", "resource.Leave"),
			want:   `rule "scope": ` + paircheck.ErrNoLinkingType.Error(),
		},
		{
			defect: "type parameter without slots",
			rule:   rule("hold", "resource.Hold", "resource.Free"),
			want: `rule "hold": ` + paircheck.ErrNoLinkingType.Error() +
				": a type parameter links only through a slot written on each side",
		},
		{
			defect: "type parameter in a slice without slots",
			rule:   rule("all", "resource.HoldAll", "resource.FreeAll"),
			want: `rule "all": ` + paircheck.ErrNoLinkingType.Error() +
				": a type parameter links only through a slot written on each side",
		},
		{
			defect: "type parameters that do not correspond",
			rule:   written(rule("crossed", "resource.Crossed", "resource.Unlock")),
			want:   `rule "crossed": ` + paircheck.ErrNoLinkingType.Error(),
		},
		{
			defect: "type parameters in different shapes",
			rule:   written(rule("shapes", "resource.HoldAll", "resource.Unlock")),
			want:   `rule "shapes": ` + paircheck.ErrNoLinkingType.Error(),
		},
	}
	for _, test := range tests {
		t.Run(test.defect, func(t *testing.T) {
			var reported recorder
			analyzer := build(t, test.rule)
			analysistest.Run(&reported, analysistest.TestData(), analyzer, "ambiguous")
			if len(reported.errors) != 1 || !strings.HasSuffix(reported.errors[0], test.want) {
				t.Errorf("errors = %q, want one ending in %q", reported.errors, test.want)
			}
		})
	}
}

func TestTypeParameter(t *testing.T) {
	rules := []paircheck.Rule{
		rule("hold", "resource.Hold", "resource.Free"),
		rule("all", "resource.HoldAll", "resource.FreeAll"),
		rule("entries", "resource.Lock", "resource.Unlock"),
	}
	for index := range rules {
		rules[index].Trigger.Slot = "argument 0"
		rules[index].Satisfiers[0].Slot = "argument 0"
	}
	analysistest.Run(t, analysistest.TestData(), build(t, rules...), "generic")
}

func TestContext(t *testing.T) {
	scope := rule("scope", "resource.Enter", "resource.Leave")
	scope.Trigger.Slot = "result 0"
	scope.Satisfiers[0].Slot = "argument 0"
	analyzer := build(t,
		rule(
			"session",
			"(*resource.Store).Begin",
			"(*resource.Session).Commit",
			"(*resource.Session).Rollback",
		),
		scope,
	)
	analysistest.Run(t, analysistest.TestData(), analyzer, "contexts")
}

func TestRelativeNames(t *testing.T) {
	analyzer := build(t,
		rule("resource", "(*./resource.Resource).Open", "(*./resource.Resource).Close"),
		rule("lease", "..Acquire", "..Release"),
	)
	analysistest.Run(t, filepath.Join(analysistest.TestData(), "module"), analyzer, "./...")
}

func TestRelativeNameOutsideAModuleFails(t *testing.T) {
	var reported recorder
	analyzer := build(
		t,
		rule("resource", "(*./resource.Resource).Open", "(*./resource.Resource).Close"),
	)
	analysistest.Run(&reported, analysistest.TestData(), analyzer, "ambiguous")
	want := `rule "resource": (./resource.Resource).Open: ` + paircheck.ErrNoModule.Error()
	if len(reported.errors) != 1 || !strings.HasSuffix(reported.errors[0], want) {
		t.Errorf("errors = %q, want one ending in %q", reported.errors, want)
	}
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

func TestRequireDefer(t *testing.T) {
	transaction := rule(
		"transaction",
		"(*resource.DB).Begin",
		"(*resource.Tx).Commit",
		"(*resource.Tx).Rollback",
	)
	transaction.RequireDefer = true
	lock := rule("lock", "(*sync.Mutex).Lock", "(*sync.Mutex).Unlock")
	lock.RequireDefer = true
	analysistest.Run(t, analysistest.TestData(), build(t, transaction, lock), "requiredefer")
}

func TestOnSuccess(t *testing.T) {
	transaction := rule(
		"transaction",
		"(*resource.DB).Begin",
		"(*resource.Tx).Commit",
		"(*resource.Tx).Rollback",
	)
	transaction.RequireDefer = true
	transaction.OnSuccess = true
	analyzer, err := paircheck.New(paircheck.Config{
		Rules:    []paircheck.Rule{transaction},
		Failures: []string{"resource.Wrap"},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	analysistest.Run(t, analysistest.TestData(), analyzer, "onsuccess")
}

func TestFunctionValue(t *testing.T) {
	analyzer := build(t,
		rule("dial", "resource.Dial", "(*resource.Conn).Close"),
		rule("acquire", "(*resource.Semaphore).Acquire", "call"),
	)
	analysistest.Run(t, analysistest.TestData(), analyzer, "funcvalue")
}

func TestDeferFirst(t *testing.T) {
	rules := []paircheck.Rule{
		rule("transaction", "(*resource.DB).Begin", "(*resource.Tx).Commit", "(*resource.Tx).Rollback"),
		rule("lock", "(*sync.Mutex).Lock", "(*sync.Mutex).Unlock"),
		rule(
			"session",
			"(*resource.Store).Begin",
			"(*resource.Session).Commit",
			"(*resource.Session).Rollback",
		),
	}
	for index := range rules {
		rules[index].DeferFirst = true
	}
	analysistest.Run(t, analysistest.TestData(), build(t, rules...), "deferfirst")
}

func TestIdempotent(t *testing.T) {
	counted := rule("counted", "(*resource.Server).Serve", "(*resource.Server).Shutdown")
	idempotent := rule("idempotent", "(*resource.Server).Serve", "(*resource.Server).Shutdown")
	idempotent.Idempotent = true
	analysistest.Run(t, analysistest.TestData(), build(t, counted, idempotent), "idempotent")
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

// written names argument 0 as the slot of the trigger and of the first
// satisfier.
func written(current paircheck.Rule) paircheck.Rule {
	current.Trigger.Slot = "argument 0"
	current.Satisfiers[0].Slot = "argument 0"
	return current
}

// recorder keeps what analysistest reports, for a case that expects the
// analysis to fail.
type recorder struct {
	errors []string
}

func (r *recorder) Errorf(format string, arguments ...any) {
	r.errors = append(r.errors, fmt.Sprintf(format, arguments...))
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
