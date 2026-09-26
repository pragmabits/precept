// Package batch has a job that is finished or cancelled.
package batch

type Job struct{}

func Start() (*Job, error) { return &Job{}, nil }

func (j *Job) Finish() error { return nil }

func (j *Job) Cancel() {}
