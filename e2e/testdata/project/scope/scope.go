// Package scope keeps a scope in the context Enter returns, which Leave ends:
// the value is the context itself, so the rule names the slot on each side.
package scope

import "context"

func Enter(ctx context.Context) context.Context { return ctx }

func Leave(ctx context.Context) {}
