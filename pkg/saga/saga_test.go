package saga_test

import (
	"context"
	"errors"
	"testing"

	"github.com/cassiomorais/payments/pkg/saga"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaga_AllStepsSucceed(t *testing.T) {
	var executed []string

	s := saga.New("test-saga").
		AddStep(saga.Step{
			Name:    "step1",
			Execute: func(ctx context.Context) error { executed = append(executed, "exec1"); return nil },
		}).
		AddStep(saga.Step{
			Name:    "step2",
			Execute: func(ctx context.Context) error { executed = append(executed, "exec2"); return nil },
		}).
		AddStep(saga.Step{
			Name:    "step3",
			Execute: func(ctx context.Context) error { executed = append(executed, "exec3"); return nil },
		})

	failedStep, err := s.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, -1, failedStep)
	assert.Equal(t, []string{"exec1", "exec2", "exec3"}, executed)
}

func TestSaga_SecondStepFails_CompensatesFirst(t *testing.T) {
	var executed []string

	s := saga.New("test-saga").
		AddStep(saga.Step{
			Name:       "step1",
			Execute:    func(ctx context.Context) error { executed = append(executed, "exec1"); return nil },
			Compensate: func(ctx context.Context) error { executed = append(executed, "comp1"); return nil },
		}).
		AddStep(saga.Step{
			Name:    "step2",
			Execute: func(ctx context.Context) error { return errors.New("step2 failed") },
			Compensate: func(ctx context.Context) error {
				// Should NOT be called because step2 didn't complete
				executed = append(executed, "comp2")
				return nil
			},
		}).
		AddStep(saga.Step{
			Name:    "step3",
			Execute: func(ctx context.Context) error { executed = append(executed, "exec3"); return nil },
		})

	failedStep, err := s.Execute(context.Background())
	assert.Error(t, err)
	assert.Equal(t, 1, failedStep)
	assert.Contains(t, err.Error(), "step2 failed")
	// Only step1 executed and got compensated. step3 never ran.
	assert.Equal(t, []string{"exec1", "comp1"}, executed)
}

func TestSaga_ThirdStepFails_CompensatesInReverse(t *testing.T) {
	var compensated []string

	s := saga.New("test-saga").
		AddStep(saga.Step{
			Name:       "step1",
			Execute:    func(ctx context.Context) error { return nil },
			Compensate: func(ctx context.Context) error { compensated = append(compensated, "comp1"); return nil },
		}).
		AddStep(saga.Step{
			Name:       "step2",
			Execute:    func(ctx context.Context) error { return nil },
			Compensate: func(ctx context.Context) error { compensated = append(compensated, "comp2"); return nil },
		}).
		AddStep(saga.Step{
			Name:    "step3",
			Execute: func(ctx context.Context) error { return errors.New("step3 failed") },
		})

	failedStep, err := s.Execute(context.Background())
	assert.Error(t, err)
	assert.Equal(t, 2, failedStep)
	// Compensation runs in reverse: step2 then step1.
	assert.Equal(t, []string{"comp2", "comp1"}, compensated)
}

func TestSaga_NoSteps(t *testing.T) {
	s := saga.New("empty")
	failedStep, err := s.Execute(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, -1, failedStep)
}

func TestSaga_MultipleCompensationErrors_AllCollected(t *testing.T) {
	s := saga.New("test-saga").
		AddStep(saga.Step{
			Name:       "step1",
			Execute:    func(ctx context.Context) error { return nil },
			Compensate: func(ctx context.Context) error { return errors.New("comp1 failed") },
		}).
		AddStep(saga.Step{
			Name:       "step2",
			Execute:    func(ctx context.Context) error { return nil },
			Compensate: func(ctx context.Context) error { return errors.New("comp2 failed") },
		}).
		AddStep(saga.Step{
			Name:    "step3",
			Execute: func(ctx context.Context) error { return errors.New("step3 failed") },
		})

	_, err := s.Execute(context.Background())
	require.Error(t, err)
	// Both compensation errors should be present (errors.Join collects all)
	assert.Contains(t, err.Error(), "comp1 failed")
	assert.Contains(t, err.Error(), "comp2 failed")
}

func TestSaga_NilCompensate(t *testing.T) {
	s := saga.New("test-saga").
		AddStep(saga.Step{
			Name:    "step1",
			Execute: func(ctx context.Context) error { return nil },
			// No compensate
		}).
		AddStep(saga.Step{
			Name:    "step2",
			Execute: func(ctx context.Context) error { return errors.New("fail") },
		})

	failedStep, err := s.Execute(context.Background())
	assert.Error(t, err)
	assert.Equal(t, 1, failedStep)
	// Should not panic despite nil Compensate.
}
