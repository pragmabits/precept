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
)

// Call names a function or method as types.Func.FullName spells it, and the
// slot carrying the value when the types leave more than one. In a
// configuration it is written either as the name alone or as an object.
type Call struct {
	Name string `json:"name"`
	Slot string `json:"slot"`
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

func (c *Call) UnmarshalText(text []byte) error {
	c.Name = string(text)
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
	return protocol{
		id:          id,
		trigger:     trigger,
		satisfiers:  satisfiers,
		openOnError: r.OpenOnError,
		coverage:    coverage,
		escapes:     r.Transfer.compile(),
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
		compiled, err := satisfier.compile()
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
}

func (c Config) compile() ([]protocol, error) {
	protocols := make([]protocol, 0, len(c.Rules))
	taken := make(map[string]bool, len(c.Rules))
	for index, rule := range c.Rules {
		compiled, err := rule.compile()
		if err != nil {
			return nil, fmt.Errorf("rule %d: %w", index, err)
		}
		if taken[compiled.id] {
			return nil, fmt.Errorf("rule %d: %w: %q", index, ErrDuplicateID, compiled.id)
		}
		taken[compiled.id] = true
		protocols = append(protocols, compiled)
	}
	return protocols, nil
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
