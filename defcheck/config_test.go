package defcheck_test

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/pragmabits/precept/defcheck"
)

func TestNewRefusesInvalidConfig(t *testing.T) {
	tests := []struct {
		defect string
		want   error
		rules  []defcheck.Rule
	}{
		{
			defect: "no pattern",
			want:   defcheck.ErrNoPattern,
			rules:  []defcheck.Rule{{Kinds: []defcheck.Kind{defcheck.KindLocalVar}}},
		},
		{
			defect: "invalid pattern",
			want:   defcheck.ErrInvalidPattern,
			rules:  []defcheck.Rule{{Pattern: "(cfg"}},
		},
		{
			defect: "unknown kind",
			want:   defcheck.ErrUnknownKind,
			rules:  []defcheck.Rule{{Pattern: "cfg", Kinds: []defcheck.Kind{"variable"}}},
		},
		{
			defect: "kind in another case",
			want:   defcheck.ErrUnknownKind,
			rules:  []defcheck.Rule{{Pattern: "cfg", Kinds: []defcheck.Kind{"Local-Var"}}},
		},
		{
			defect: "unknown kind left out",
			want:   defcheck.ErrUnknownKind,
			rules:  []defcheck.Rule{{Pattern: "cfg", Kinds: []defcheck.Kind{"-variable"}}},
		},
		{
			defect: "kinds named and left out",
			want:   defcheck.ErrMixedKinds,
			rules: []defcheck.Rule{{
				Pattern: "cfg",
				Kinds:   []defcheck.Kind{defcheck.KindFunction, "-" + defcheck.KindLabel},
			}},
		},
		{
			defect: "every kind left out",
			want:   defcheck.ErrNoKindLeft,
			rules:  []defcheck.Rule{{Pattern: "cfg", Kinds: leftOut(every()...)}},
		},
		{
			defect: "duplicate rule",
			want:   defcheck.ErrDuplicateRule,
			rules: []defcheck.Rule{
				{Pattern: "cfg", Kinds: []defcheck.Kind{defcheck.KindField}, Message: "avoid cfg"},
				{Pattern: "cfg", Kinds: []defcheck.Kind{defcheck.KindField}, Message: "avoid cfg"},
			},
		},
		{
			defect: "duplicate kinds in another order",
			want:   defcheck.ErrDuplicateRule,
			rules: []defcheck.Rule{
				{Pattern: "cfg", Kinds: []defcheck.Kind{defcheck.KindField, defcheck.KindLocalVar}},
				{Pattern: "cfg", Kinds: []defcheck.Kind{defcheck.KindLocalVar, defcheck.KindField}},
			},
		},
		{
			defect: "duplicate kinds repeated",
			want:   defcheck.ErrDuplicateRule,
			rules: []defcheck.Rule{
				{Pattern: "cfg", Kinds: []defcheck.Kind{defcheck.KindField}},
				{Pattern: "cfg", Kinds: []defcheck.Kind{defcheck.KindField, defcheck.KindField}},
			},
		},
		{
			defect: "duplicate kinds left out",
			want:   defcheck.ErrDuplicateRule,
			rules:  []defcheck.Rule{{Pattern: "cfg"}, {Pattern: "cfg", Kinds: every()}},
		},
		{
			defect: "duplicate kinds named and left out",
			want:   defcheck.ErrDuplicateRule,
			rules: []defcheck.Rule{
				{Pattern: "cfg", Kinds: every()[1:]},
				{Pattern: "cfg", Kinds: leftOut(defcheck.KindFunction)},
			},
		},
		{
			defect: "duplicate kinds left out twice",
			want:   defcheck.ErrDuplicateRule,
			rules: []defcheck.Rule{
				{Pattern: "cfg", Kinds: leftOut(defcheck.KindLabel)},
				{Pattern: "cfg", Kinds: leftOut(defcheck.KindLabel, defcheck.KindLabel)},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.defect, func(t *testing.T) {
			_, err := defcheck.New(defcheck.Config{Rules: test.rules})
			if !errors.Is(err, test.want) {
				t.Errorf("New() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestNewNamesTheRuleItRefuses(t *testing.T) {
	config := defcheck.Config{Rules: []defcheck.Rule{{Pattern: "cfg"}, {Pattern: "(cfg"}}}
	_, err := defcheck.New(config)
	if err == nil || !strings.HasPrefix(err.Error(), "rule 1: ") {
		t.Errorf("New() error = %v, want one starting with %q", err, "rule 1: ")
	}
}

func TestNewNamesTheRuleADuplicateRepeats(t *testing.T) {
	config := defcheck.Config{Rules: []defcheck.Rule{
		{Pattern: "cfg"},
		{Pattern: "mgr"},
		{Pattern: "cfg"},
	}}
	_, err := defcheck.New(config)
	want := "rule 2: " + defcheck.ErrDuplicateRule.Error() + ": rule 0"
	if err == nil || err.Error() != want {
		t.Errorf("New() error = %v, want %q", err, want)
	}
}

func TestNewAcceptsRulesThatDiffer(t *testing.T) {
	rules := []defcheck.Rule{
		{Pattern: "cfg"},
		{Pattern: "cfg", Message: "avoid cfg"},
		{Pattern: "cfg", Kinds: []defcheck.Kind{defcheck.KindField}},
		{Pattern: "^cfg$"},
		{Pattern: "cfg", Kinds: every()[1:]},
		{Pattern: "cfg", Kinds: leftOut(defcheck.KindLabel)},
	}
	if _, err := defcheck.New(defcheck.Config{Rules: rules}); err != nil {
		t.Errorf("New() error = %v, want nil", err)
	}
}

func TestNewAcceptsEveryKind(t *testing.T) {
	for _, kind := range every() {
		t.Run(string(kind), func(t *testing.T) {
			rule := defcheck.Rule{Pattern: "cfg", Kinds: []defcheck.Kind{kind}}
			if _, err := defcheck.New(defcheck.Config{Rules: []defcheck.Rule{rule}}); err != nil {
				t.Errorf("New() error = %v, want nil", err)
			}
		})
	}
}

func TestNewAcceptsEveryKindLeftOut(t *testing.T) {
	for _, kind := range every() {
		t.Run(string(kind), func(t *testing.T) {
			rule := defcheck.Rule{Pattern: "cfg", Kinds: leftOut(kind)}
			if _, err := defcheck.New(defcheck.Config{Rules: []defcheck.Rule{rule}}); err != nil {
				t.Errorf("New() error = %v, want nil", err)
			}
		})
	}
}

func TestNewQuotesTheKindAsWritten(t *testing.T) {
	config := defcheck.Config{Rules: []defcheck.Rule{{Pattern: "cfg", Kinds: leftOut("variable")}}}
	_, err := defcheck.New(config)
	if err == nil || !strings.HasSuffix(err.Error(), `: "-variable"`) {
		t.Errorf("New() error = %v, want one ending in %q", err, `: "-variable"`)
	}
}

func TestNewAcceptsNoRule(t *testing.T) {
	if _, err := defcheck.New(defcheck.Config{}); err != nil {
		t.Errorf("New() error = %v, want nil", err)
	}
}

func TestRuleDecodes(t *testing.T) {
	const document = `{
		"pattern": "^cfg$",
		"kinds": ["local-var", "parameter", "field"],
		"message": "avoid the cfg abbreviation"
	}`
	var decoded defcheck.Rule
	if err := json.Unmarshal([]byte(document), &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	want := defcheck.Rule{
		Pattern: "^cfg$",
		Kinds: []defcheck.Kind{
			defcheck.KindLocalVar,
			defcheck.KindParameter,
			defcheck.KindField,
		},
		Message: "avoid the cfg abbreviation",
	}
	if decoded.Pattern != want.Pattern || decoded.Message != want.Message ||
		!slices.Equal(decoded.Kinds, want.Kinds) {
		t.Errorf("Unmarshal() = %+v, want %+v", decoded, want)
	}
}

// every is each kind a rule may name.
func every() []defcheck.Kind {
	return []defcheck.Kind{
		defcheck.KindFunction,
		defcheck.KindMethod,
		defcheck.KindPackageVar,
		defcheck.KindLocalVar,
		defcheck.KindConstant,
		defcheck.KindField,
		defcheck.KindParameter,
		defcheck.KindReceiver,
		defcheck.KindResult,
		defcheck.KindType,
		defcheck.KindTypeParameter,
		defcheck.KindLabel,
	}
}

// leftOut is kinds, each written after a -, as a rule leaves a kind out.
func leftOut(kinds ...defcheck.Kind) []defcheck.Kind {
	written := make([]defcheck.Kind, 0, len(kinds))
	for _, kind := range kinds {
		written = append(written, "-"+kind)
	}
	return written
}
