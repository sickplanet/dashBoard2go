package queue

import (
	"context"
	"time"
)

// JobStatus defines the state of a queue item
type JobStatus string

const (
	StatusPending    JobStatus = "PENDING"
	StatusProcessing JobStatus = "PROCESSING"
	StatusCompleted  JobStatus = "COMPLETED"
	StatusFailed     JobStatus = "FAILED"
)

// Job defines a task injected into SQLite by the Core, picked up by the Worker
type Job struct {
	ID         int       `json:"id"`
	Action     string    `json:"action"`  // e.g. "ADD_FTP_USER", "RESTART_NGINX", "CREATE_VHOST"
	Payload    string    `json:"payload"` // JSON encoded data required for the wrapper
	Status     JobStatus `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	FinishedAt time.Time `json:"finished_at,omitempty"`
	ErrorLog   string    `json:"error_log,omitempty"` // Written by the Worker if it crashes
}

// JobQueue defines the interface for interacting with the database queue
type JobQueue interface {
	// Push is used by dashboard2go-core to defer execution
	Push(ctx context.Context, action string, payload string) (int, error)

	// Pop is used by dashboard2go-worker to grab the next pending task, locking it
	Pop(ctx context.Context) (*Job, error)

	// MarkComplete is used by the worker upon success
	MarkComplete(ctx context.Context, jobID int) error

	// MarkFailed is used by the worker if the wrapper throws an error
	MarkFailed(ctx context.Context, jobID int, errStr string) error
}
