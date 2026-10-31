package task

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Task represents a background task
type Task struct {
	ID        string
	Type      string
	Status    string
	Progress  int
	Total     int
	Result    interface{}
	Error     error
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Queue manages background tasks
type Queue struct {
	mu      sync.RWMutex
	tasks   map[string]*Task
	workers int
	jobs    chan func()
}

// NewQueue creates a new task queue
func NewQueue(workers int) *Queue {
	q := &Queue{
		tasks:   make(map[string]*Task),
		workers: workers,
		jobs:    make(chan func(), 100),
	}

	// Start workers
	for i := 0; i < workers; i++ {
		go q.worker()
	}

	return q
}

// worker processes jobs
func (q *Queue) worker() {
	for job := range q.jobs {
		job()
	}
}

// Submit submits a task
func (q *Queue) Submit(ctx context.Context, taskType string, fn func(context.Context, *Task) error) (string, error) {
	task := &Task{
		ID:        fmt.Sprintf("task_%d", time.Now().UnixNano()),
		Type:      taskType,
		Status:    "pending",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	q.mu.Lock()
	q.tasks[task.ID] = task
	q.mu.Unlock()

	// Submit job
	q.jobs <- func() {
		task.Status = "running"
		task.UpdatedAt = time.Now()

		if err := fn(ctx, task); err != nil {
			task.Status = "failed"
			task.Error = err
		} else {
			task.Status = "completed"
		}

		task.UpdatedAt = time.Now()
	}

	return task.ID, nil
}

// Get gets a task by ID
func (q *Queue) Get(id string) (*Task, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	task, ok := q.tasks[id]
	if !ok {
		return nil, fmt.Errorf("task not found: %s", id)
	}

	return task, nil
}

// List lists all tasks
func (q *Queue) List() []*Task {
	q.mu.RLock()
	defer q.mu.RUnlock()

	tasks := make([]*Task, 0, len(q.tasks))
	for _, task := range q.tasks {
		tasks = append(tasks, task)
	}

	return tasks
}

// Cancel cancels a task
func (q *Queue) Cancel(id string) error {
	// TODO: Implement task cancellation
	return fmt.Errorf("not implemented")
}

// UpdateProgress updates task progress
func (q *Queue) UpdateProgress(task *Task, progress, total int) {
	task.Progress = progress
	task.Total = total
	task.UpdatedAt = time.Now()
}
