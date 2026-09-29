package paircheck

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrNoID            = errors.New("rule has no id")
	ErrDuplicateID     = errors.New("rule id is already taken")
	ErrNoTrigger       = errors.New("rule has no trigger")
	ErrNoSatisfiers    = errors.New("rule has no satisfiers")
	ErrEmptySatisfier  = errors.New("satisfier has no name")
	ErrInvalidName     = errors.New("name is not a qualified function or method name")
	ErrInvalidSlot     = errors.New(`slot is not "receiver", "argument N" or "result N"`)
	ErrUnknownCoverage = errors.New(`deferred-closure is not "any-path", "every-path" or "none"`)
	ErrUnknownTransfer = errors.New(`transfer is not "all", "none" or an object`)
	ErrCallSlot        = errors.New("the call satisfier takes no slot")
	ErrNoType          = errors.New("implementation has no name")
	ErrInvalidType     = errors.New("name is not a qualified type name")
)

// callSatisfier is the satisfier that calls the value itself, when the value is
// a function. It never collides with a name: a name is qualified.
const callSatisfier = "call"

// Call names a function or method as types.Func.FullName spells it, and the
// slot carrying the value when the types leave more than one. In a
// configuration it is written either as the name alone or as an object.
type Call struct {
	Name string `json:"name"`
	Slot string `json:"slot"`
}

// compileSatisfier is compile, where the word call also stands for the
// satisfier that calls the value itself, which has no slot to name.
func (c Call) compileSatisfier() (side, error) {
	if c.Name != callSatisfier {
		return c.compile()
	}
	if c.Slot != "" {
		return side{}, ErrCallSlot
	}
	return side{called: true}, nil
}

func (c Call) compile() (side, error) {
	name, err := parseName(c.Name)
	if err != nil {
		return side{}, err
	}
	position, err := parseSlot(c.Slot)
	if err != nil {
		return side{}, err
	}
	return side{name: name, slot: position}, nil
}

// UnmarshalJSON takes the name alone or the object. The text form goes through
// UnmarshalText, which is also what golangci-lint's own decoder calls.
func (c *Call) UnmarshalJSON(data []byte) error {
	type plain Call
	return decodeTextOrObject(data, c.UnmarshalText, (*plain)(c), ErrInvalidName)
}

// UnmarshalText takes the name alone, and leaves the slot to deduction.
func (c *Call) UnmarshalText(text []byte) error {
	c.Name = string(text)
	return nil
}

// Implementation is a concrete type a rule written on an interface applies to
// as well, with the methods of the type in place of those of the interfaces it
// implements, and the id its findings carry: the rule's when it names none. In
// a configuration it is written either as the name of the type alone, as
// example.com/project/session.Store, or as an object.
type Implementation struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

// compile is the implementation of the rule whose id is rule: a type name, as
// types.TypeName spells it with its package's path, the pointer and the type
// parameters left out.
func (i Implementation) compile(rule string) (implementation, error) {
	if i.Name == "" {
		return implementation{}, ErrNoType
	}
	written := withoutTypeParameters(strings.TrimPrefix(i.Name, "*"))
	name, err := parseName(written)
	if err != nil || strings.HasPrefix(written, "(") {
		return implementation{}, fmt.Errorf("%w: %q", ErrInvalidType, i.Name)
	}
	id := strings.TrimSpace(i.ID)
	if id == "" {
		id = rule
	}
	return implementation{name: name, id: id}, nil
}

// UnmarshalJSON takes the name alone or the object.
func (i *Implementation) UnmarshalJSON(data []byte) error {
	type plain Implementation
	return decodeTextOrObject(data, i.UnmarshalText, (*plain)(i), ErrInvalidType)
}

// UnmarshalText takes the name alone, and leaves the id to the rule.
func (i *Implementation) UnmarshalText(text []byte) error {
	i.Name = string(text)
	return nil
}

// Coverage is how much of a deferred closure has to call a satisfier for the
// closure to discharge the obligation.
type Coverage string

const (
	CoverageAnyPath   Coverage = "any-path"
	CoverageEveryPath Coverage = "every-path"
	CoverageNone      Coverage = "none"
)

func (c Coverage) normalize() (Coverage, error) {
	switch c {
	case "":
		return CoverageAnyPath, nil
	case CoverageAnyPath, CoverageEveryPath, CoverageNone:
		return c, nil
	}
	return "", fmt.Errorf("%w: %q", ErrUnknownCoverage, string(c))
}

// Transfer says, for each way a value leaves a function, whether the
// obligation leaves with it. A field left nil is true.
type Transfer struct {
	Return   *bool `json:"return"`
	Store    *bool `json:"store"`
	Argument *bool `json:"argument"`
}

func (t Transfer) compile() escapes {
	return escapes{
		returned: enabled(t.Return),
		stored:   enabled(t.Store),
		passed:   enabled(t.Argument),
	}
}

// UnmarshalJSON takes "all", "none" or the object.
func (t *Transfer) UnmarshalJSON(data []byte) error {
	type plain Transfer
	return decodeTextOrObject(data, t.UnmarshalText, (*plain)(t), ErrUnknownTransfer)
}

// UnmarshalText sets every way at once: "all" or "none".
func (t *Transfer) UnmarshalText(text []byte) error {
	var every bool
	switch string(text) {
	case "all":
		every = true
	case "none":
		every = false
	default:
		return fmt.Errorf("%w: %q", ErrUnknownTransfer, string(text))
	}
	returned, stored, passed := every, every, every
	*t = Transfer{Return: &returned, Store: &stored, Argument: &passed}
	return nil
}

// Rule declares that a call to Trigger opens an obligation that a call to one
// of Satisfiers must discharge before the function exits.
type Rule struct {
	ID         string `json:"id"`
	Trigger    Call   `json:"trigger"`
	Satisfiers []Call `json:"satisfiers"`

	// OpenOnError opens the obligation even on the path where the trigger
	// returned a non-nil error.
	OpenOnError bool `json:"open-on-error"`

	// DeferredClosure says when a satisfier inside a deferred closure
	// discharges the obligation. Empty is CoverageAnyPath.
	DeferredClosure Coverage `json:"deferred-closure"`

	// Transfer says through which exits the value takes the obligation with
	// it out of the function.
	Transfer Transfer `json:"transfer"`

	// RequireDefer counts only a deferred satisfier: one called on the path
	// does not discharge the obligation, since a panic before it would leave
	// the obligation open.
	RequireDefer bool `json:"require-defer"`

	// Idempotent makes a trigger on a value whose obligation is open open no
	// other: a second Serve on a running server needs no second Shutdown.
	Idempotent bool `json:"idempotent"`

	// DeferFirst requires a deferred satisfier before any other call once the
	// obligation opens: a panic in a call before it would leave the obligation
	// open.
	DeferFirst bool `json:"defer-first"`

	// OnSuccess requires, on every return that does not hand back a failure, a
	// satisfier called on the path: a deferred one alone does not say how the
	// obligation ends when the function succeeds.
	OnSuccess bool `json:"on-success"`

	// Implementations are the concrete types the rule applies to as well, when
	// it is written on an interface.
	Implementations []Implementation `json:"implementations"`
}

func (r Rule) compile() (protocol, error) {
	id := strings.TrimSpace(r.ID)
	if id == "" {
		return protocol{}, ErrNoID
	}
	if r.Trigger.Name == "" {
		return protocol{}, fmt.Errorf("%q: %w", id, ErrNoTrigger)
	}
	trigger, err := r.Trigger.compile()
	if err != nil {
		return protocol{}, fmt.Errorf("%q: trigger: %w", id, err)
	}
	satisfiers, err := r.compileSatisfiers()
	if err != nil {
		return protocol{}, fmt.Errorf("%q: %w", id, err)
	}
	coverage, err := r.DeferredClosure.normalize()
	if err != nil {
		return protocol{}, fmt.Errorf("%q: %w", id, err)
	}
	implementations := make([]implementation, 0, len(r.Implementations))
	for index, each := range r.Implementations {
		compiled, err := each.compile(id)
		if err != nil {
			return protocol{}, fmt.Errorf("%q: implementation %d: %w", id, index, err)
		}
		implementations = append(implementations, compiled)
	}
	return protocol{
		id:              id,
		trigger:         trigger,
		satisfiers:      satisfiers,
		openOnError:     r.OpenOnError,
		coverage:        coverage,
		escapes:         r.Transfer.compile(),
		requireDefer:    r.RequireDefer,
		idempotent:      r.Idempotent,
		deferFirst:      r.DeferFirst,
		onSuccess:       r.OnSuccess,
		implementations: implementations,
	}, nil
}

func (r Rule) compileSatisfiers() ([]side, error) {
	if len(r.Satisfiers) == 0 {
		return nil, ErrNoSatisfiers
	}
	satisfiers := make([]side, 0, len(r.Satisfiers))
	for index, satisfier := range r.Satisfiers {
		if satisfier.Name == "" {
			return nil, fmt.Errorf("satisfier %d: %w", index, ErrEmptySatisfier)
		}
		compiled, err := satisfier.compileSatisfier()
		if err != nil {
			return nil, fmt.Errorf("satisfier %d: %w", index, err)
		}
		satisfiers = append(satisfiers, compiled)
	}
	return satisfiers, nil
}

// Config is the set of rules the analyzer enforces.
type Config struct {
	Rules []Rule `json:"rules"`

	// Failures names the functions and methods whose call returns a non-nil
	// error, besides errors.New and fmt.Errorf: a return that hands back what
	// one of them returned is a failure to a rule that is on-success.
	Failures []string `json:"failures"`
}

// compile validates every rule into a protocol. A protocol that is on-success
// carries the failures it reads a return by.
func (c Config) compile() ([]protocol, error) {
	failures, err := c.compileFailures()
	if err != nil {
		return nil, err
	}
	protocols := make([]protocol, 0, len(c.Rules))
	taken := make(map[string]bool, len(c.Rules))
	for index, rule := range c.Rules {
		compiled, err := rule.compile()
		if err != nil {
			return nil, fmt.Errorf("rule %d: %w", index, err)
		}
		if err := claim(taken, compiled.id); err != nil {
			return nil, fmt.Errorf("rule %d: %w", index, err)
		}
		for _, each := range rule.Implementations {
			if err := claim(taken, strings.TrimSpace(each.ID)); err != nil {
				return nil, fmt.Errorf("rule %d: %s: %w", index, each.Name, err)
			}
		}
		if compiled.onSuccess {
			compiled.failures = failures
		}
		protocols = append(protocols, compiled)
	}
	return protocols, nil
}

// claim takes id, which no rule and no implementation may take twice. An empty
// id is an implementation's that takes its rule's.
func claim(taken map[string]bool, id string) error {
	if id == "" {
		return nil
	}
	if taken[id] {
		return fmt.Errorf("%w: %q", ErrDuplicateID, id)
	}
	taken[id] = true
	return nil
}

func (c Config) compileFailures() ([]qualifiedName, error) {
	failures := make([]qualifiedName, 0, len(c.Failures))
	for index, text := range c.Failures {
		name, err := parseName(text)
		if err != nil {
			return nil, fmt.Errorf("failure %d: %w", index, err)
		}
		failures = append(failures, name)
	}
	return failures, nil
}

// decodeTextOrObject decodes a JSON string through text and an object into
// object, refusing a key object does not have. Anything else is invalid.
func decodeTextOrObject(data []byte, text func([]byte) error, object any, invalid error) error {
	data = bytes.TrimSpace(data)
	if bytes.HasPrefix(data, []byte(`"`)) {
		var content string
		if err := json.Unmarshal(data, &content); err != nil {
			return err
		}
		return text([]byte(content))
	}
	if !bytes.HasPrefix(data, []byte("{")) {
		return fmt.Errorf("%w: %s", invalid, data)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(object)
}

func enabled(flag *bool) bool {
	return flag == nil || *flag
}
