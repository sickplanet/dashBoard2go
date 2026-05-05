package queue

import (
"context"
"database/sql"
"fmt"
"time"
)

// SQLiteQueue implements the JobQueue interface for a SQLite backend.
type SQLiteQueue struct {
db *sql.DB
}

// NewSQLiteQueue creates a new SQLite queue and ensures the jobs table exists.
func NewSQLiteQueue(db *sql.DB) (*SQLiteQueue, error) {
q := &SQLiteQueue{db: db}
if err := q.InitTable(); err != nil {
return nil, fmt.Errorf("failed to init job table: %v", err)
}
return q, nil
}

func (q *SQLiteQueue) InitTable() error {
query := `CREATE TABLE IF NOT EXISTS jobs (
id INTEGER PRIMARY KEY AUTOINCREMENT,
action TEXT NOT NULL,
payload TEXT NOT NULL,
status TEXT NOT NULL DEFAULT 'PENDING',
created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
finished_at DATETIME,
error_log TEXT
);`
_, err := q.db.Exec(query)
return err
}

func (q *SQLiteQueue) Push(ctx context.Context, action string, payload string) (int, error) {
query := `INSERT INTO jobs (action, payload, status, created_at) VALUES (?, ?, 'PENDING', ?)`
res, err := q.db.ExecContext(ctx, query, action, payload, time.Now())
if err != nil {
return 0, err
}
id, err := res.LastInsertId()
return int(id), err
}

func (q *SQLiteQueue) Pop(ctx context.Context) (*Job, error) {
// SQLite doesn't have a direct SKIP LOCKED, but with WAL and careful updates we can emulate it.
// We'll use a transaction to select a pending job and mark it as PROCESSING immediately.
tx, err := q.db.BeginTx(ctx, nil)
if err != nil {
return nil, err
}
defer tx.Rollback()

query := `SELECT id, action, payload, status, created_at FROM jobs WHERE status = 'PENDING' ORDER BY id ASC LIMIT 1`
row := tx.QueryRowContext(ctx, query)

var job Job
var status string
var createdAt time.Time

err = row.Scan(&job.ID, &job.Action, &job.Payload, &status, &createdAt)
if err != nil {
if err == sql.ErrNoRows {
return nil, nil // No jobs pending
}
return nil, err
}
job.Status = JobStatus(status)
job.CreatedAt = createdAt

// Mark as processing
updateQuery := `UPDATE jobs SET status = 'PROCESSING' WHERE id = ?`
_, err = tx.ExecContext(ctx, updateQuery, job.ID)
if err != nil {
return nil, fmt.Errorf("failed to mark job as processing: %v", err)
}

if err := tx.Commit(); err != nil {
return nil, fmt.Errorf("failed to commit pop transaction: %v", err)
}

return &job, nil
}

func (q *SQLiteQueue) MarkComplete(ctx context.Context, jobID int) error {
query := `UPDATE jobs SET status = 'COMPLETED', finished_at = ? WHERE id = ?`
_, err := q.db.ExecContext(ctx, query, time.Now(), jobID)
return err
}

func (q *SQLiteQueue) MarkFailed(ctx context.Context, jobID int, errStr string) error {
query := `UPDATE jobs SET status = 'FAILED', finished_at = ?, error_log = ? WHERE id = ?`
_, err := q.db.ExecContext(ctx, query, time.Now(), errStr, jobID)
return err
}
