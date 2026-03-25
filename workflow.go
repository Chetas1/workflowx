package salvador

import "context"

// StepFunc is the function signature for a workflow step.
// It receives the context and a pointer to the current task.
// Return nil to advance to the next step, or an error to fail the task.
type StepFunc func(ctx context.Context, task *Task) error

type step struct {
	name string
	fn   StepFunc
}

// Workflow defines a named sequence of steps to execute for a task.
type Workflow struct {
	name  string
	steps []step
}

// NewWorkflow creates a new workflow with the given name.
// The name must match the task name when submitting tasks.
func NewWorkflow(name string) *Workflow {
	return &Workflow{name: name}
}

// Step appends a step to the workflow. Steps run in the order they are added.
func (w *Workflow) Step(name string, fn StepFunc) *Workflow {
	w.steps = append(w.steps, step{name: name, fn: fn})
	return w
}

// Steps returns the number of steps in the workflow.
func (w *Workflow) Steps() int {
	return len(w.steps)
}

// Name returns the workflow name.
func (w *Workflow) Name() string {
	return w.name
}
