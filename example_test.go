package salvador_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/workflowx/salvador"
)

func Example() {
	// 1. Connect to Redis
	store := salvador.NewRedisStore(nil) // pass your *redis.Client here

	// 2. Define a workflow with ordered steps
	wf := salvador.NewWorkflow("process-order")

	wf.Step("validate", func(ctx context.Context, task *salvador.Task) error {
		var order map[string]any
		if err := json.Unmarshal(task.Body, &order); err != nil {
			return fmt.Errorf("invalid order payload: %w", err)
		}
		log.Printf("validated order: %v", order)
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

	// 3. Create the engine and register the workflow
	engine := salvador.NewEngine(store,
		salvador.WithConcurrency(3),
	)
	engine.Register(wf)

	// 4. Submit a task
	ctx := context.Background()
	body, _ := json.Marshal(map[string]any{
		"item":     "widget",
		"quantity": 5,
	})
	taskID, err := engine.Submit(ctx, "process-order", body)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("submitted task: %s", taskID)

	// 5. Start processing (blocks until context is cancelled)
	// ctx, cancel := context.WithCancel(context.Background())
	// defer cancel()
	// engine.Start(ctx)
}
