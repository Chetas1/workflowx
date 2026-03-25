package salvador

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Engine runs workflows by polling Redis for pending tasks and executing steps in order.
type Engine struct {
	store     Store
	workflows map[string]*Workflow
	mu        sync.RWMutex
	pollInterval time.Duration
	concurrency  int
	logger       *log.Logger
}

// EngineOption configures the Engine.
type EngineOption func(*Engine)

// WithPollInterval sets how often the engine checks for new tasks. Defaults to 1s.
func WithPollInterval(d time.Duration) EngineOption {
	return func(e *Engine) {
		e.pollInterval = d
	}
}

// WithConcurrency sets the number of concurrent workers per workflow. Defaults to 1.
func WithConcurrency(n int) EngineOption {
	return func(e *Engine) {
		if n > 0 {
			e.concurrency = n
		}
	}
}

// WithLogger sets a custom logger. Defaults to log.Default().
func WithLogger(l *log.Logger) EngineOption {
	return func(e *Engine) {
		e.logger = l
	}
}

// NewEngine creates a new workflow engine backed by the given store.
func NewEngine(store Store, opts ...EngineOption) *Engine {
	e := &Engine{
		store:        store,
		workflows:    make(map[string]*Workflow),
		pollInterval: time.Second,
		concurrency:  1,
		logger:       log.Default(),
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Register adds a workflow to the engine. Panics if a workflow with the same name
// is already registered.
func (e *Engine) Register(w *Workflow) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.workflows[w.name]; exists {
		panic(fmt.Sprintf("salvador: workflow %q already registered", w.name))
	}
	e.workflows[w.name] = w
}

// Submit creates a new task and enqueues it for the named workflow.
// Returns the task ID.
func (e *Engine) Submit(ctx context.Context, workflowName string, body []byte) (string, error) {
	e.mu.RLock()
	_, ok := e.workflows[workflowName]
	e.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("salvador: unknown workflow %q", workflowName)
	}

	task := &Task{
		ID:        uuid.New().String(),
		Name:      workflowName,
		Body:      body,
		Status:    TaskStatusPending,
		Step:      0,
		CreatedAt: time.Now().UTC(),
	}

	if err := e.store.Save(ctx, task); err != nil {
		return "", fmt.Errorf("save task: %w", err)
	}
	if err := e.store.Enqueue(ctx, workflowName, task.ID); err != nil {
		return "", fmt.Errorf("enqueue task: %w", err)
	}
	return task.ID, nil
}

// Start begins processing tasks for all registered workflows.
// It blocks until the context is cancelled.
func (e *Engine) Start(ctx context.Context) error {
	e.mu.RLock()
	names := make([]string, 0, len(e.workflows))
	for name := range e.workflows {
		names = append(names, name)
	}
	e.mu.RUnlock()

	var wg sync.WaitGroup
	for _, name := range names {
		for i := 0; i < e.concurrency; i++ {
			wg.Add(1)
			go func(wfName string) {
				defer wg.Done()
				e.poll(ctx, wfName)
			}(name)
		}
	}

	wg.Wait()
	return ctx.Err()
}

func (e *Engine) poll(ctx context.Context, workflowName string) {
	ticker := time.NewTicker(e.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			taskID, err := e.store.Dequeue(ctx, workflowName)
			if err != nil {
				e.logger.Printf("salvador: dequeue error for %q: %v", workflowName, err)
				continue
			}
			if taskID == "" {
				continue
			}
			e.execute(ctx, workflowName, taskID)
		}
	}
}

func (e *Engine) execute(ctx context.Context, workflowName string, taskID string) {
	e.mu.RLock()
	wf := e.workflows[workflowName]
	e.mu.RUnlock()

	task, err := e.store.Get(ctx, taskID)
	if err != nil {
		e.logger.Printf("salvador: get task %q: %v", taskID, err)
		return
	}

	task.Status = TaskStatusRunning
	if err := e.store.Save(ctx, task); err != nil {
		e.logger.Printf("salvador: save task %q: %v", taskID, err)
		return
	}

	for i := task.Step; i < len(wf.steps); i++ {
		s := wf.steps[i]
		e.logger.Printf("salvador: task %s running step %d/%d %q", taskID, i+1, len(wf.steps), s.name)

		if err := s.fn(ctx, task); err != nil {
			task.Status = TaskStatusFailed
			task.Error = fmt.Sprintf("step %q: %v", s.name, err)
			task.Step = i
			if saveErr := e.store.Save(ctx, task); saveErr != nil {
				e.logger.Printf("salvador: save failed task %q: %v", taskID, saveErr)
			}
			return
		}

		task.Step = i + 1
		if err := e.store.Save(ctx, task); err != nil {
			e.logger.Printf("salvador: save task %q after step %q: %v", taskID, s.name, err)
			return
		}
	}

	task.Status = TaskStatusComplete
	if err := e.store.Save(ctx, task); err != nil {
		e.logger.Printf("salvador: save completed task %q: %v", taskID, err)
	}
}

// GetTask retrieves a task by ID from the store.
func (e *Engine) GetTask(ctx context.Context, id string) (*Task, error) {
	return e.store.Get(ctx, id)
}
