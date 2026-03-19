package workflow

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// JobStatus represents the current execution state for a workflow job.
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusSucceeded JobStatus = "succeeded"
	JobStatusFailed    JobStatus = "failed"
)

// Job defines a unit of work that must be idempotent.
type Job interface {
	Name() string
	IdempotencyKey() string
	Execute(ctx context.Context) error
}

// Result captures execution details for a job.
type Result struct {
	JobName         string
	IdempotencyKey  string
	Status          JobStatus
	Error           string
	StartedAt       time.Time
	CompletedAt     time.Time
	ExecutionNumber int
}

// Engine runs jobs with idempotency tracking.
type Engine struct {
	mu      sync.Mutex
	history map[string]Result
}

// NewEngine initializes an in-memory workflow engine skeleton.
func NewEngine() *Engine {
	return &Engine{history: make(map[string]Result)}
}

// Run executes a job if it has not completed successfully with the same idempotency key.
func (e *Engine) Run(ctx context.Context, job Job) (Result, error) {
	if job == nil {
		return Result{}, fmt.Errorf("job is required")
	}

	if job.Name() == "" {
		return Result{}, fmt.Errorf("job name is required")
	}

	key := job.IdempotencyKey()
	if key == "" {
		return Result{}, fmt.Errorf("idempotency key is required")
	}

	e.mu.Lock()
	if previous, ok := e.history[key]; ok && previous.Status == JobStatusSucceeded {
		e.mu.Unlock()
		return previous, nil
	}

	executionNumber := 1
	if previous, ok := e.history[key]; ok {
		executionNumber = previous.ExecutionNumber + 1
	}

	started := time.Now().UTC()
	e.history[key] = Result{
		JobName:         job.Name(),
		IdempotencyKey:  key,
		Status:          JobStatusRunning,
		StartedAt:       started,
		ExecutionNumber: executionNumber,
	}
	e.mu.Unlock()

	err := job.Execute(ctx)
	completed := time.Now().UTC()

	e.mu.Lock()
	defer e.mu.Unlock()

	result := Result{
		JobName:         job.Name(),
		IdempotencyKey:  key,
		StartedAt:       started,
		CompletedAt:     completed,
		ExecutionNumber: executionNumber,
	}

	if err != nil {
		result.Status = JobStatusFailed
		result.Error = err.Error()
		e.history[key] = result
		return result, err
	}

	result.Status = JobStatusSucceeded
	e.history[key] = result
	return result, nil
}

// GetResult retrieves the latest known result for an idempotency key.
func (e *Engine) GetResult(idempotencyKey string) (Result, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	result, ok := e.history[idempotencyKey]
	return result, ok
}
