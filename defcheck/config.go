package defcheck

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

var (
	ErrNoPattern      = errors.New("rule has no pattern")
	ErrInvalidPattern = errors.New("pattern is not a regular expression")
	ErrUnknownKind    = errors.New("kind is not " + listed(every))
	ErrMixedKinds     = errors.New("rule both names kinds and leaves kinds out")
	ErrNoKindLeft     = errors.New("rule leaves out every kind")
	ErrDuplicateRule  = errors.New("rule repeats the pattern, kinds and message of another rule")
)

// Kind is a kind of declaration, as go/types classifies the object a
// declaration defines.
type Kind string

const (
	KindFunction      Kind = "function"
	KindMethod        Kind = "method"
	KindPackageVar    Kind = "package-var"
	KindLocalVar      Kind = "local-var"
	KindConstant      Kind = "constant"
	KindField         Kind = "field"
	KindParameter     Kind = "parameter"
	KindReceiver      Kind = "receiver"
	KindResult        Kind = "result"
	KindType          Kind = "type"
	KindTypeParameter Kind = "type-parameter"
	KindLabel         Kind = "label"
)

// every is each kind, in the order a rule's kinds are normalized to.
var every = []Kind{
	KindFunction,
	KindMethod,
	KindPackageVar,
	KindLocalVar,
	KindConstant,
	KindField,
	KindParameter,
	KindReceiver,
	KindResult,
	KindType,
	KindTypeParameter,
	KindLabel,
}

// Rule forbids the names Pattern matches in the declarations of Kinds: the
// kinds it names, or every kind but those it writes after a -, and every kind
// when it writes none. Message, when written, replaces the pattern and the
// kind in the diagnostic.
type Rule struct {
	Pattern string `json:"pattern"`
	Kinds   []Kind `json:"kinds"`
	Message string `json:"message"`
}

// compile validates the rule into a prohibition. An empty pattern is refused:
// it is what a rule that leaves the key out decodes to, and it matches every
// name.
func (r Rule) compile() (prohibition, error) {
	if r.Pattern == "" {
		return prohibition{}, ErrNoPattern
	}
	pattern, err := regexp.Compile(r.Pattern)
	if err != nil {
		return prohibition{}, fmt.Errorf("%w: %w", ErrInvalidPattern, err)
	}
	kinds, err := normalize(r.Kinds)
	if err != nil {
		return prohibition{}, err
	}
	return prohibition{pattern: pattern, kinds: kinds, message: r.Message}, nil
}

// Config is the set of rules the analyzer enforces.
type Config struct {
	Rules []Rule `json:"rules"`
}

// compile validates every rule into a prohibition, in the order written. A
// rule whose pattern, kinds and message another rule already has is refused:
// the two would report every declaration twice alike.
func (c Config) compile() ([]prohibition, error) {
	prohibitions := make([]prohibition, 0, len(c.Rules))
	taken := make(map[string]int, len(c.Rules))
	for index, rule := range c.Rules {
		compiled, err := rule.compile()
		if err != nil {
			return nil, fmt.Errorf("rule %d: %w", index, err)
		}
		key := compiled.key()
		if first, repeated := taken[key]; repeated {
			return nil, fmt.Errorf("rule %d: %w: rule %d", index, ErrDuplicateRule, first)
		}
		taken[key] = index
		prohibitions = append(prohibitions, compiled)
	}
	return prohibitions, nil
}

// normalize is the kinds written, each once, in the order of every. None
// written is every kind, and kinds each written after a - are every kind but
// those, refused when they are every kind.
func normalize(written []Kind) ([]Kind, error) {
	if len(written) == 0 {
		return slices.Clone(every), nil
	}
	named, leftOut, err := partition(written)
	if err != nil {
		return nil, err
	}
	kinds := slices.DeleteFunc(slices.Clone(every), func(kind Kind) bool {
		if len(named) > 0 {
			return !slices.Contains(named, kind)
		}
		return slices.Contains(leftOut, kind)
	})
	if len(kinds) == 0 {
		return nil, ErrNoKindLeft
	}
	return kinds, nil
}

// partition splits written into the kinds it names and those it leaves out,
// written after a -. A kind that is not one of every is refused, as written,
// and so is a list that both names kinds and leaves kinds out.
func partition(written []Kind) ([]Kind, []Kind, error) {
	var named, leftOut []Kind
	for _, kind := range written {
		name, isLeftOut := strings.CutPrefix(string(kind), "-")
		if !slices.Contains(every, Kind(name)) {
			return nil, nil, fmt.Errorf("%w: %q", ErrUnknownKind, string(kind))
		}
		if isLeftOut {
			leftOut = append(leftOut, Kind(name))
		} else {
			named = append(named, Kind(name))
		}
	}
	if len(named) > 0 && len(leftOut) > 0 {
		return nil, nil, fmt.Errorf("%w: %q and %q", ErrMixedKinds, named[0], "-"+leftOut[0])
	}
	return named, leftOut, nil
}

// listed is kinds as a sentence lists them: "a, b or c".
func listed(kinds []Kind) string {
	names := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		names = append(names, string(kind))
	}
	last := len(names) - 1
	return strings.Join(names[:last], ", ") + " or " + names[last]
}
