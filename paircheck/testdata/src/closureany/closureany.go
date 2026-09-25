package closureany

import (
	"errors"

	"resource"
)

var errFailed = errors.New("failed")

func unconditional(r *resource.Resource) {
	r.Open()
	defer func() {
		r.Close()
	}()
}

func conditional(r *resource.Resource, failed bool) {
	r.Open()
	defer func() {
		if failed {
			r.Close()
		}
	}()
}

func withoutSatisfier(r *resource.Resource) {
	r.Open() // want `\[resource\] Open requires Close on r before function exit`
	defer func() {
		r.Use()
	}()
}

func transaction(db *resource.DB, fail bool) (err error) {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if fail {
		return errFailed
	}
	return tx.Commit()
}
