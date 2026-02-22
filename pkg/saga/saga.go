package saga

import (
	"context"
	"errors"
	"fmt"
)

// Step represents a single step in a saga with an execute and compensate function.
type Step struct {
	Name       string
	Execute    func(ctx context.Context) error
	Compensate func(ctx context.Context) error
}

// Saga orchestrates a series of steps with automatic compensation on failure.
type Saga struct {
	name  string
	steps []Step
}

// New creates a new saga with the given name.
func New(name string) *Saga {
	return &Saga{name: name}
}

// AddStep adds a step to the saga.
func (s *Saga) AddStep(step Step) *Saga {
	s.steps = append(s.steps, step)
	return s
}

// Execute runs all saga steps sequentially.
// If any step fails, it compensates all previously completed steps in reverse order.
// Returns the index of the failed step and the error, or -1 and nil on success.
func (s *Saga) Execute(ctx context.Context) (failedStep int, err error) {
	completed := make([]int, 0, len(s.steps))

	for i, step := range s.steps {
		if err := step.Execute(ctx); err != nil {
			// Compensate in reverse order
			compErr := s.compensate(ctx, completed)
			if compErr != nil {
				return i, fmt.Errorf("saga %s: step %q failed (%w), compensation also failed: %v", s.name, step.Name, err, compErr)
			}
			return i, fmt.Errorf("saga %s: step %q failed: %w", s.name, step.Name, err)
		}
		completed = append(completed, i)
	}

	return -1, nil
}

func (s *Saga) compensate(ctx context.Context, completedIndexes []int) error {
	var errs []error
	// Compensate in reverse order
	for i := len(completedIndexes) - 1; i >= 0; i-- {
		step := s.steps[completedIndexes[i]]
		if step.Compensate == nil {
			continue
		}
		if err := step.Compensate(ctx); err != nil {
			errs = append(errs, fmt.Errorf("compensate step %q: %w", step.Name, err))
		}
	}
	return errors.Join(errs...)
}
