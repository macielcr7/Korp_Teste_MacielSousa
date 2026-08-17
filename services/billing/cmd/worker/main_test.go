package main

import (
	"context"
	"errors"
	"testing"
	"time"

	closureusecase "github.com/macielcr7/korp-teste/services/billing/internal/application/usecase/billing/closureoperation"
)

type blockingProcessor struct{}

func (blockingProcessor) Execute(ctx context.Context) (closureusecase.ProcessOutput, error) {
	<-ctx.Done()
	return closureusecase.ProcessOutput{}, ctx.Err()
}

func TestWorkerRetryDelay(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		base     time.Duration
		failures int
		want     time.Duration
	}{
		{name: "first failure", base: time.Second, failures: 1, want: time.Second},
		{name: "second failure", base: time.Second, failures: 2, want: 2 * time.Second},
		{name: "fourth failure", base: time.Second, failures: 4, want: 8 * time.Second},
		{name: "capped delay", base: time.Second, failures: 10, want: maximumWorkerRetryDelay},
		{name: "base above cap", base: time.Minute, failures: 3, want: time.Minute},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := workerRetryDelay(test.base, test.failures); got != test.want {
				t.Fatalf("workerRetryDelay() = %s, want %s", got, test.want)
			}
		})
	}
}

func TestWaitForWorkerStopsWhenContextIsCancelled(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if waitForWorker(ctx, time.Minute) {
		t.Fatal("waitForWorker() = true, want false for cancelled context")
	}
}

func TestExecuteAttemptLimitsProcessingTime(t *testing.T) {
	t.Parallel()
	startedAt := time.Now()

	_, err := executeAttempt(context.Background(), blockingProcessor{}, 20*time.Millisecond)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("executeAttempt() error = %v, want context deadline exceeded", err)
	}
	if elapsed := time.Since(startedAt); elapsed > time.Second {
		t.Fatalf("executeAttempt() took %s, want less than one second", elapsed)
	}
}
