# Salvador

A Redis-backed task workflow engine for Go. Define tasks with ordered steps, persist them in Redis, and execute them reliably.

## Install

```bash
go get github.com/workflowx/salvador
```

## Quick Start

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/redis/go-redis/v9"
	"github.com/workflowx/salvador"
)

func main() {
	// Connect to Redis
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	store := salvador.NewRedisStore(rdb)

	// Define a workflow with ordered steps
	wf := salvador.NewWorkflow("process-order")

	wf.Step("validate", func(ctx context.Context, task *salvador.Task) error {
		var order map[string]any
		if err := json.Unmarshal(task.Body, &order); err != nil {
			return fmt.Errorf("invalid order: %w", err)
		}
		log.Printf("order validated: %v", order)
		return nil
	})

	wf.Step("charge-payment", func(ctx context.Context, task *salvador.Task) error {
		log.Printf("charging payment for task %s", task.ID)
		return nil
	})

	wf.Step("ship", func(ctx context.Context, task *salvador.Task) error {
		log.Printf("shipping order for task %s", task.ID)
		return nil
	})

	// Create the engine and register the workflow
	engine := salvador.NewEngine(store, salvador.WithConcurrency(3))
	engine.Register(wf)

	ctx := context.Background()

	// Submit a task
	body, _ := json.Marshal(map[string]any{"item": "widget", "quantity": 5})
	taskID, err := engine.Submit(ctx, "process-order", body)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("submitted task: %s", taskID)

	// Start processing (blocks until context is cancelled)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	engine.Start(ctx)
}
```

## Concepts

### Task

A task is a unit of work with a name, body (arbitrary bytes), and status. Tasks are persisted in Redis and transition through these statuses:

`pending` -> `running` -> `completed` or `failed`

### Workflow

A workflow is a named sequence of steps. Each step is a function with the signature:

```go
func(ctx context.Context, task *salvador.Task) error
```

Steps run in the order they are added. If a step returns an error, the task is marked as failed and subsequent steps are skipped.

### Engine

The engine polls Redis for pending tasks and executes the matching workflow's steps. Configure it with options:

```go
engine := salvador.NewEngine(store,
	salvador.WithPollInterval(2 * time.Second), // how often to check for tasks (default: 1s)
	salvador.WithConcurrency(5),                // workers per workflow (default: 1)
	salvador.WithLogger(myLogger),              // custom logger (default: log.Default())
)
```

## Custom Store

The `Store` interface can be implemented for backends other than Redis:

```go
type Store interface {
	Save(ctx context.Context, task *Task) error
	Get(ctx context.Context, id string) (*Task, error)
	Enqueue(ctx context.Context, workflowName string, taskID string) error
	Dequeue(ctx context.Context, workflowName string) (string, error)
}
```

### Redis Store Options

```go
store := salvador.NewRedisStore(rdb, salvador.WithKeyPrefix("myapp:"))
```

All Redis keys are prefixed with `salvador:` by default. Use `WithKeyPrefix` to customize.

## Checking Task Status

```go
task, err := engine.GetTask(ctx, taskID)
if err != nil {
	log.Fatal(err)
}
fmt.Printf("status: %s, step: %d\n", task.Status, task.Step)
if task.Status == salvador.TaskStatusFailed {
	fmt.Printf("error: %s\n", task.Error)
}
```

## License

See [LICENSE](LICENSE).
