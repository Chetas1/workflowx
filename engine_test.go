package salvador_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/workflowx/salvador"
)

// memStore is an in-memory Store for testing without Redis.
type memStore struct {
	mu     sync.Mutex
	tasks  map[string]*salvador.Task
	queues map[string][]string
}

func newMemStore() *memStore {
	return &memStore{
		tasks:  make(map[string]*salvador.Task),
		queues: make(map[string][]string),
	}
}

func (m *memStore) Save(_ context.Context, task *salvador.Task) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// deep copy to avoid aliasing
	cp := *task
	cp.Body = make([]byte, len(task.Body))
	copy(cp.Body, task.Body)
	m.tasks[task.ID] = &cp
	return nil
}

func (m *memStore) Get(_ context.Context, id string) (*salvador.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tasks[id]
	if !ok {
		return nil, fmt.Errorf("task %q not found", id)
	}
	cp := *t
	return &cp, nil
}

func (m *memStore) Enqueue(_ context.Context, workflowName string, taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queues[workflowName] = append(m.queues[workflowName], taskID)
	return nil
}

func (m *memStore) Dequeue(_ context.Context, workflowName string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	q := m.queues[workflowName]
	if len(q) == 0 {
		return "", nil
	}
	id := q[0]
	m.queues[workflowName] = q[1:]
	return id, nil
}

func TestEngineRunsStepsInOrder(t *testing.T) {
	store := newMemStore()

	var mu sync.Mutex
	var order []string

	wf := salvador.NewWorkflow("test-wf")
	wf.Step("step-1", func(ctx context.Context, task *salvador.Task) error {
		mu.Lock()
		order = append(order, "step-1")
		mu.Unlock()
		return nil
	})
	wf.Step("step-2", func(ctx context.Context, task *salvador.Task) error {
		mu.Lock()
		order = append(order, "step-2")
		mu.Unlock()
		return nil
	})
	wf.Step("step-3", func(ctx context.Context, task *salvador.Task) error {
		mu.Lock()
		order = append(order, "step-3")
		mu.Unlock()
		return nil
	})

	engine := salvador.NewEngine(store, salvador.WithPollInterval(50*time.Millisecond))
	engine.Register(wf)

	ctx := context.Background()
	taskID, err := engine.Submit(ctx, "test-wf", []byte(`{"hello":"world"}`))
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	// Run engine briefly
	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	engine.Start(ctx)

	// Verify step order
	mu.Lock()
	defer mu.Unlock()
	if len(order) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(order))
	}
	for i, expected := range []string{"step-1", "step-2", "step-3"} {
		if order[i] != expected {
			t.Errorf("step %d: got %q, want %q", i, order[i], expected)
		}
	}

	// Verify task status
	task, err := store.Get(context.Background(), taskID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if task.Status != salvador.TaskStatusComplete {
		t.Errorf("task status: got %q, want %q", task.Status, salvador.TaskStatusComplete)
	}
}

func TestEngineStepFailure(t *testing.T) {
	store := newMemStore()

	wf := salvador.NewWorkflow("fail-wf")
	wf.Step("ok-step", func(ctx context.Context, task *salvador.Task) error {
		return nil
	})
	wf.Step("bad-step", func(ctx context.Context, task *salvador.Task) error {
		return fmt.Errorf("something went wrong")
	})
	wf.Step("never-runs", func(ctx context.Context, task *salvador.Task) error {
		t.Error("this step should not run")
		return nil
	})

	engine := salvador.NewEngine(store, salvador.WithPollInterval(50*time.Millisecond))
	engine.Register(wf)

	ctx := context.Background()
	taskID, err := engine.Submit(ctx, "fail-wf", []byte(`{}`))
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	engine.Start(ctx)

	task, err := store.Get(context.Background(), taskID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if task.Status != salvador.TaskStatusFailed {
		t.Errorf("task status: got %q, want %q", task.Status, salvador.TaskStatusFailed)
	}
	if task.Step != 1 {
		t.Errorf("task step: got %d, want 1 (failed at second step)", task.Step)
	}
}

func TestSubmitUnknownWorkflow(t *testing.T) {
	store := newMemStore()
	engine := salvador.NewEngine(store)

	_, err := engine.Submit(context.Background(), "nonexistent", []byte(`{}`))
	if err == nil {
		t.Fatal("expected error for unknown workflow")
	}
}
