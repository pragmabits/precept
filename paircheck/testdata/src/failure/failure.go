package failure

import (
	"errors"

	"resource"
)

func checked(r *resource.Resource) error {
	if err := r.Start(); err != nil {
		return err
	}
	defer r.Stop()
	return nil
}

func checkedAsNil(r *resource.Resource) error {
	err := r.Start()
	if err == nil {
		defer r.Stop()
		return nil
	}
	return err
}

func neverChecked(r *resource.Resource) {
	_ = r.Start() // want `\[start\] Start requires Stop on r before function exit`
}

func checkedButLeaked(r *resource.Resource) error {
	if err := r.Start(); err != nil { // want `\[start\] Start requires Stop on r before function exit`
		return err
	}
	return nil
}

func unrecognized(r *resource.Resource) error {
	err := r.Start()
	if errors.Is(err, resource.ErrBusy) {
		return err
	}
	defer r.Stop()
	return nil
}

func unrecognizedIsUncertain(r *resource.Resource) error {
	err := r.Start()
	if errors.Is(err, resource.ErrBusy) {
		return err
	}
	return nil
}

func recognizedAfterUnrecognized(r *resource.Resource) error {
	err := r.Start() // want `\[start\] Start requires Stop on r before function exit`
	if errors.Is(err, resource.ErrBusy) {
		return err
	}
	if err != nil {
		return err
	}
	return nil
}

func resultChecked(db *resource.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	return tx.Commit()
}

func resultLeaked(db *resource.DB) error {
	tx, err := db.Begin() // want `\[transaction\] Begin requires one of \[Commit, Rollback\] on tx before function exit`
	if err != nil {
		return err
	}
	_ = tx
	return nil
}

func storedError(r *resource.Resource) (err error) {
	defer func() {
		_ = err
	}()
	if err = r.Start(); err != nil {
		return err
	}
	defer r.Stop()
	return nil
}

func step() error { return nil }

// reassignedError keeps the error in a variable a deferred closure reads, and
// stores another error in it before a check: that check is not about Start.
func reassignedError(r *resource.Resource) (err error) {
	defer func() {
		_ = err
	}()
	if err = r.Start(); err != nil { // want `\[start\] Start requires Stop on r before function exit`
		return err
	}
	if err = step(); err != nil {
		return err
	}
	r.Stop()
	return nil
}

// reassignedOnOnePath stores another error on one path only: past the retry,
// the check reads Start's error on one path and step's on the other.
func reassignedOnOnePath(r *resource.Resource, retry bool) (err error) {
	defer func() {
		_ = err
	}()
	if err = r.Start(); err != nil { // want `\[start\] Start requires Stop on r before function exit`
		return err
	}
	if retry {
		err = step()
	}
	if err != nil {
		return err
	}
	r.Stop()
	return nil
}
