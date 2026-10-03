// Package imposed declares names the language or an interface imposes, which
// defcheck leaves out, beside names of the same spelling nothing imposes.
package imposed

import (
	"encoding/json"
	"fmt"
	"io"
)

func init() {}

func main() {} // want `"main" is forbidden by pattern`

// value implements error, fmt.Stringer, json.Marshaler, io.Closer through its
// pointer, and store, whose packages the package declares or imports; and
// sort.Interface, whose package it does not import.
type value struct{}

func (value) Error() string { return "" }

func (value) String() string { return fmt.Sprint() }

func (value) MarshalJSON() ([]byte, error) { return json.Marshal(nil) }

func (*value) Close() error { return nil }

func (value) Get() string { return "" }

func (value) Len() int { return 0 } // want `"Len" is forbidden by pattern`

func (value) Less(int, int) bool { return false } // want `"Less" is forbidden by pattern`

func (value) Swap(int, int) {} // want `"Swap" is forbidden by pattern`

// Read has the name of io.Reader's method, and not its signature.
func (value) Read() int { return 0 } // want `"Read" is forbidden by pattern`

// store names its method here, where it is reported.
type store interface {
	Get() string // want `"Get" is forbidden by pattern`
}

// box is generic, and implements fmt.Stringer.
type box[T any] struct{ item T }

func (b box[T]) String() string { return fmt.Sprint(b.item) }

// other implements nothing: its String is not fmt.Stringer's.
type other struct{}

func (other) String() int { return 0 } // want `"String" is forbidden by pattern`

// String is a function, which implements no interface.
func String() string { return "" } // want `"String" is forbidden by pattern`

var _ io.Closer = (*value)(nil)
