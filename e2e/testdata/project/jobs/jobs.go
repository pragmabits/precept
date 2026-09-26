// Package jobs uses the batch API under five rules: batch-any, batch-every and
// batch-none, one per deferred-closure value, and batch-strict-every and
// batch-strict-none, the last two with require-defer.
package jobs

import "example.com/project/batch"

func step() error { return nil }

// CancelOnError cancels in a deferred closure only when the job failed, and
// finishes at the end.
func CancelOnError() (err error) {
	job, err := batch.Start() // want `\[batch-every\] Start requires one of \[Finish, Cancel\] on job before function exit` `\[batch-none\] Start requires one of \[Finish, Cancel\] on job before function exit` `\[batch-strict-every\] Start requires one of \[Finish, Cancel\] on job before function exit` `\[batch-strict-none\] Start requires one of \[Finish, Cancel\] on job before function exit`
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			job.Cancel()
		}
	}()
	if err = step(); err != nil {
		return err
	}
	return job.Finish()
}

// CancelAlways cancels in a deferred closure on every path.
func CancelAlways() error {
	job, err := batch.Start() // want `\[batch-none\] Start requires one of \[Finish, Cancel\] on job before function exit` `\[batch-strict-none\] Start requires one of \[Finish, Cancel\] on job before function exit`
	if err != nil {
		return err
	}
	defer func() {
		job.Cancel()
	}()
	if err := step(); err != nil {
		return err
	}
	return job.Finish()
}

// FinishByHand cancels or finishes on every path, with no defer.
func FinishByHand() error {
	job, err := batch.Start() // want `\[batch-strict-every\] Start requires one of \[Finish, Cancel\] on job before function exit` `\[batch-strict-none\] Start requires one of \[Finish, Cancel\] on job before function exit`
	if err != nil {
		return err
	}
	if err := step(); err != nil {
		job.Cancel()
		return err
	}
	return job.Finish()
}
