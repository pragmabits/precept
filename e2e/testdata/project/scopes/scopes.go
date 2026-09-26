// Package scopes uses scope under the scope rule.
package scopes

import (
	"context"

	"example.com/project/scope"
)

// EarlyReturn returns on a cancelled context with the scope open.
func EarlyReturn(ctx context.Context) {
	inner := scope.Enter(ctx) // want `\[scope\] Enter requires Leave on inner before function exit`
	if inner.Err() != nil {
		return
	}
	scope.Leave(inner)
}

// Deferred leaves the scope in a defer.
func Deferred(ctx context.Context) {
	inner := scope.Enter(ctx)
	defer scope.Leave(inner)
}
