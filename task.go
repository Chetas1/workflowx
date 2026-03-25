package salvador

import "time"

type TaskStatus string

const (
	TaskStatusPending  TaskStatus = "pending"
	TaskStatusRunning  TaskStatus = "running"
	TaskStatusComplete TaskStatus = "completed"
	TaskStatusFailed   TaskStatus = "failed"
)

// Task represents a unit of work submitted to a workflow.
type Task struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Body      []byte            `json:"body"`
	Status    TaskStatus        `json:"status"`
	Step      int               `json:"step"`
	Error     string            `json:"error,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
	Meta      map[string]string `json:"meta,omitempty"`
}
