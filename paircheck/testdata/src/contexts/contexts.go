package contexts

import (
	"context"

	"resource"
)

func session(ctx context.Context, store *resource.Store) error {
	current, err := store.Begin(ctx) // want `\[session\] Begin requires one of \[Commit, Rollback\] on current before function exit`
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return current.Commit(ctx)
}

func scope(ctx context.Context) {
	inner := resource.Enter(ctx) // want `\[scope\] Enter requires Leave on inner before function exit`
	_ = inner
}
