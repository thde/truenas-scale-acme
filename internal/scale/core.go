package scale

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-querystring/query"
)

type Job struct {
	ID           int      `json:"id,omitempty"`
	Method       string   `json:"method,omitempty"`
	Transient    bool     `json:"transient,omitempty"`
	Abortable    bool     `json:"abortable,omitempty"`
	LogsPath     any      `json:"logs_path,omitempty"`
	LogsExcerpt  any      `json:"logs_excerpt,omitempty"`
	Progress     Progress `json:"progress,omitempty"`
	Result       any      `json:"result,omitempty"`
	Error        string   `json:"error,omitempty"`
	Exception    string   `json:"exception,omitempty"`
	State        string   `json:"state,omitempty"`
	TimeStarted  Date     `json:"time_started,omitempty"`
	TimeFinished Date     `json:"time_finished,omitempty"`
}

type Progress struct {
	Percent     uint8  `json:"percent,omitempty"`
	Description string `json:"description,omitempty"`
	Extra       any    `json:"extra,omitempty"`
}

type Timestamp struct {
	time.Time
}

func (t *Timestamp) UnmarshalJSON(data []byte) error {
	var i int64
	if err := json.Unmarshal(data, &i); err != nil {
		return err
	}
	t.Time = time.Unix(i, 0)
	return nil
}

type Date struct {
	Date Timestamp `json:"$date,omitempty"`
}

type JobsParams struct {
	ID    int `url:"id,omitempty"`
	Limit int `url:"limit,omitempty"`
}

// CoreGetJobs returns a list of Jobs.
func (c *Client) CoreGetJobs(ctx context.Context, params JobsParams) ([]Job, error) {
	v, err := query.Values(params)
	if err != nil {
		return nil, err
	}

	req, err := c.newRequest(ctx, http.MethodGet, "core/get_jobs", v, nil)
	if err != nil {
		return nil, fmt.Errorf("get jobs error: %w", err)
	}

	jobs := []Job{}
	_, err = c.doJSON(req, &jobs)
	return jobs, err
}

// CoreGetJob returns a Job.
func (c *Client) CoreGetJob(ctx context.Context, id int) (*Job, error) {
	jobs, err := c.CoreGetJobs(ctx, JobsParams{ID: id, Limit: 1})
	if err != nil {
		return nil, err
	}
	if len(jobs) == 0 {
		return nil, fmt.Errorf("job %d not found", id)
	}

	return &jobs[0], err
}

// CoreJobWait waits for a job to finish.
func (c *Client) CoreJobWait(ctx context.Context, id int) (*Job, error) {
	timeout := time.After(10 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return nil, errors.New("timed out")
		case <-ticker.C:
			job, err := c.CoreGetJob(ctx, id)
			if err != nil {
				return nil, err
			}

			if job.State != "RUNNING" {
				return job, nil
			}
		}
	}
}

// doJob exectures a request that returns a Job and waits for it to finish.
func (c *Client) doJob(req *http.Request) (*Job, error) {
	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	if resp.Body == nil {
		return nil, nil
	}
	defer resp.Body.Close()

	i, err := parseJobID(resp.Body)
	if err != nil {
		return nil, err
	}

	job, err := c.CoreJobWait(req.Context(), i)
	if err != nil {
		return nil, err
	}

	if job.Error != "" {
		return nil, errors.New(job.Error)
	}

	return job, err
}

func parseJobID(body io.Reader) (int, error) {
	buf := strings.Builder{}
	_, err := io.Copy(&buf, body)
	if err != nil {
		return 0, fmt.Errorf("reading job body failed: %w", err)
	}

	return strconv.Atoi(buf.String())
}
