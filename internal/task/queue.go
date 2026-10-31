package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("task not found")
var ErrClosed = errors.New("task queue is closed")

type Task struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"`
	Status       string    `json:"status"`
	Progress     int       `json:"progress"`
	Total        int       `json:"total"`
	Result       any       `json:"result,omitempty"`
	Error        error     `json:"-"`
	ErrorMessage string    `json:"error,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Queue struct {
	mu      sync.RWMutex
	tasks   map[string]*Task
	cancels map[string]context.CancelFunc
	jobs    chan func()
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func NewQueue(workers int) *Queue {
	if workers < 1 {
		workers = 1
	}
	ctx, cancel := context.WithCancel(context.Background())
	q := &Queue{tasks: map[string]*Task{}, cancels: map[string]context.CancelFunc{}, jobs: make(chan func(), 100), ctx: ctx, cancel: cancel}
	for i := 0; i < workers; i++ {
		q.wg.Add(1)
		go func() {
			defer q.wg.Done()
			for {
				select {
				case <-q.ctx.Done():
					return
				case job := <-q.jobs:
					job()
				}
			}
		}()
	}
	return q
}
func (q *Queue) Submit(ctx context.Context, typ string, fn func(context.Context, *Task) error) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if fn == nil {
		return "", fmt.Errorf("task callback is required")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	jobCtx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(q.ctx, cancel)
	now := time.Now().UTC()
	value := &Task{ID: uuid.NewString(), Type: typ, Status: "pending", CreatedAt: now, UpdatedAt: now}
	q.mu.Lock()
	if q.ctx.Err() != nil {
		q.mu.Unlock()
		stop()
		cancel()
		return "", ErrClosed
	}
	// Keep a bounded history while never evicting live tasks.
	if len(q.tasks) >= 1000 {
		var oldest *Task
		for _, t := range q.tasks {
			_, active := q.cancels[t.ID]
			if !active && t.Status != "pending" && t.Status != "running" && (oldest == nil || t.CreatedAt.Before(oldest.CreatedAt)) {
				oldest = t
			}
		}
		if oldest != nil {
			delete(q.tasks, oldest.ID)
		}
	}
	q.tasks[value.ID] = value
	q.cancels[value.ID] = cancel
	q.mu.Unlock()
	job := func() {
		defer stop()
		defer cancel()
		q.mu.Lock()
		local := *value
		if jobCtx.Err() == nil {
			value.Status = "running"
			value.UpdatedAt = time.Now().UTC()
		}
		q.mu.Unlock()
		var err error
		if jobCtx.Err() != nil {
			err = jobCtx.Err()
		} else {
			err = invoke(jobCtx, &local, fn)
		}
		q.mu.Lock()
		defer q.mu.Unlock()
		value.Result = local.Result
		value.UpdatedAt = time.Now().UTC()
		delete(q.cancels, value.ID)
		if jobCtx.Err() != nil {
			value.Status = "cancelled"
			value.Error = jobCtx.Err()
		} else if err != nil {
			value.Status = "failed"
			value.Error = err
		} else {
			value.Status = "completed"
			value.Progress = value.Total
		}
		if value.Error != nil {
			value.ErrorMessage = value.Error.Error()
		}
	}
	select {
	case q.jobs <- job:
		return value.ID, nil
	case <-ctx.Done():
		q.drop(value.ID)
		stop()
		cancel()
		return "", ctx.Err()
	case <-q.ctx.Done():
		q.drop(value.ID)
		stop()
		cancel()
		return "", ErrClosed
	}
}
func invoke(ctx context.Context, t *Task, fn func(context.Context, *Task) error) (err error) {
	defer func() {
		if value := recover(); value != nil {
			err = fmt.Errorf("task panicked: %v", value)
		}
	}()
	return fn(ctx, t)
}
func (q *Queue) drop(id string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	delete(q.tasks, id)
	delete(q.cancels, id)
}
func snapshot(value *Task) *Task {
	copy := *value
	if value.Result != nil {
		if data, err := json.Marshal(value.Result); err == nil {
			copy.Result = nil
			_ = json.Unmarshal(data, &copy.Result)
		}
	}
	return &copy
}
func (q *Queue) Get(id string) (*Task, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	value, ok := q.tasks[id]
	if !ok {
		return nil, ErrNotFound
	}
	return snapshot(value), nil
}
func (q *Queue) List() []*Task {
	q.mu.RLock()
	defer q.mu.RUnlock()
	values := []*Task{}
	for _, value := range q.tasks {
		values = append(values, snapshot(value))
	}
	sort.Slice(values, func(i, j int) bool { return values[i].CreatedAt.Before(values[j].CreatedAt) })
	return values
}
func (q *Queue) Cancel(id string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	value, ok := q.tasks[id]
	if !ok {
		return ErrNotFound
	}
	if cancel, ok := q.cancels[id]; ok {
		cancel()
		value.Status = "cancelled"
		value.Error = context.Canceled
		value.ErrorMessage = context.Canceled.Error()
		value.UpdatedAt = time.Now().UTC()
	}
	return nil
}
func (q *Queue) UpdateProgress(value *Task, progress, total int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if stored, ok := q.tasks[value.ID]; ok {
		if total < 0 {
			total = 0
		}
		if progress < 0 {
			progress = 0
		}
		if progress > total {
			progress = total
		}
		stored.Progress, stored.Total = progress, total
		stored.UpdatedAt = time.Now().UTC()
	}
}
func (q *Queue) Close() error {
	q.cancel()
	q.mu.Lock()
	for id, cancel := range q.cancels {
		cancel()
		value := q.tasks[id]
		value.Status = "cancelled"
		value.Error = context.Canceled
		value.ErrorMessage = context.Canceled.Error()
		value.UpdatedAt = time.Now().UTC()
	}
	q.mu.Unlock()
	q.wg.Wait()
	return nil
}
