package truenas

import (
	"context"
	"fmt"
	"time"
)

const (
	jobPollInterval = 500 * time.Millisecond
	jobWaitTimeout  = 60 * time.Second
)

// Job represents a TrueNAS background job as returned by core.get_jobs.
// Methods decorated as jobs return a job ID rather than their result; the
// result is delivered asynchronously once the job reaches a terminal state.
type Job struct {
	ID     int    `json:"id"`
	Method string `json:"method"`
	State  string `json:"state"`
	Error  string `json:"error"`
}

// jobQueryOptions are the query-options for core.get_jobs.
type jobQueryOptions struct {
	Limit int `json:"limit,omitempty"`
}

// getJob fetches a single job by ID via core.get_jobs.
func (c *Client) getJob(ctx context.Context, id int) (*Job, error) {
	var jobs []Job
	err := c.withReconnect(ctx, func() error {
		var err error
		jobs, err = c.a.CoreGetJobs(ctx, [][]any{{"id", "=", id}}, jobQueryOptions{Limit: 1})
		return err
	})
	if err != nil {
		return nil, err
	}
	if len(jobs) == 0 {
		return nil, fmt.Errorf("job %d not found", id)
	}
	return &jobs[0], nil
}

// waitForJob polls core.get_jobs until the job reaches a terminal state.
// It returns an error if the job fails or is aborted, or if the context is
// cancelled or jobWaitTimeout elapses.
func (c *Client) waitForJob(ctx context.Context, id int) (*Job, error) {
	ctx, cancel := context.WithTimeout(ctx, jobWaitTimeout)
	defer cancel()

	ticker := time.NewTicker(jobPollInterval)
	defer ticker.Stop()

	for {
		job, err := c.getJob(ctx, id)
		if err != nil {
			return nil, err
		}

		switch job.State {
		case "SUCCESS":
			return job, nil
		case "FAILED", "ABORTED":
			return nil, fmt.Errorf("job %d %s: %s", id, job.State, job.Error)
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("waiting for job %d: %w", id, ctx.Err())
		case <-ticker.C:
		}
	}
}
