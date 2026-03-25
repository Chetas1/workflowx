package salvador

import "context"

// Store is the interface for task persistence.
// The default implementation uses Redis.
type Store interface {
	// Save persists a task. It creates or updates based on task ID.
	Save(ctx context.Context, task *Task) error

	// Get retrieves a task by ID.
	Get(ctx context.Context, id string) (*Task, error)

	// Enqueue adds a task ID to the workflow's pending queue.
	Enqueue(ctx context.Context, workflowName string, taskID string) error

	// Dequeue removes and returns the next task ID from the workflow's pending queue.
	// Returns empty string and nil error if the queue is empty.
	Dequeue(ctx context.Context, workflowName string) (string, error)
}
