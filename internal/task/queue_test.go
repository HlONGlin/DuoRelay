package task

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func waitTask(t *testing.T, q *Queue, id, status string) *Task {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		value, err := q.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		if value.Status == status {
			return value
		}
		time.Sleep(time.Millisecond)
	}
	value, _ := q.Get(id)
	t.Fatalf("task did not reach %s: %#v", status, value)
	return nil
}
func TestProgressSnapshotsAndPanic(t *testing.T) {
	q := NewQueue(0)
	defer q.Close()
	id, err := q.Submit(context.Background(), "test", func(ctx context.Context, value *Task) error {
		q.UpdateProgress(value, 1, 2)
		value.Result = map[string]any{"value": 1}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	value := waitTask(t, q, id, "completed")
	if value.Progress != 2 || value.Total != 2 {
		t.Fatalf("bad progress: %#v", value)
	}
	value.Result.(map[string]any)["value"] = 9
	copy, _ := q.Get(id)
	if copy.Result.(map[string]any)["value"] != float64(1) {
		t.Fatal("snapshot mutated queue state")
	}
	id, err = q.Submit(context.Background(), "panic", func(context.Context, *Task) error { panic("test") })
	if err != nil {
		t.Fatal(err)
	}
	if value := waitTask(t, q, id, "failed"); value.ErrorMessage == "" {
		t.Fatal("panic was lost")
	}
	if _, err := q.Get("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}
func TestCancellationAndClose(t *testing.T) {
	q := NewQueue(1)
	started := make(chan struct{})
	id, err := q.Submit(context.Background(), "cancel", func(ctx context.Context, _ *Task) error { close(started); <-ctx.Done(); return ctx.Err() })
	if err != nil {
		t.Fatal(err)
	}
	<-started
	if err := q.Cancel(id); err != nil {
		t.Fatal(err)
	}
	waitTask(t, q, id, "cancelled")
	if err := q.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := q.Submit(context.Background(), "late", func(context.Context, *Task) error { return nil }); !errors.Is(err, ErrClosed) {
		t.Fatal("closed queue accepted work")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := q.Submit(ctx, "cancelled", func(context.Context, *Task) error { return nil }); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
func TestConcurrentProgressAndReads(t *testing.T) {
	q := NewQueue(2)
	defer q.Close()
	var wg sync.WaitGroup
	id, err := q.Submit(context.Background(), "progress", func(ctx context.Context, value *Task) error {
		for i := 0; i < 500; i++ {
			q.UpdateProgress(value, i, 500)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				q.Get(id)
				q.List()
			}
		}()
	}
	wg.Wait()
	waitTask(t, q, id, "completed")
}
