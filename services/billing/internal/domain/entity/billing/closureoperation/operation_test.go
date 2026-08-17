package closureoperation

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOperationTransitions(t *testing.T) {
	now := time.Now().UTC()
	op, err := Rehydrate(Snapshot{
		ID: "op", InvoiceID: "inv", IdempotencyKey: "key", CommandID: "cmd",
		Status: StatusProcessing, Attempts: 1, NextAttemptAt: now, CreatedAt: now, UpdatedAt: now,
	})
	require.NoError(t, err)
	require.NoError(t, op.MarkRetrying("inventory unavailable", now.Add(time.Second), now))
	require.Equal(t, StatusRetrying, op.Status())
	require.ErrorIs(t, op.MarkCompleted(now), ErrInvalidTransition)
}

func TestNewRequiresIdempotencyKey(t *testing.T) {
	_, err := New("op", "inv", "", "cmd", time.Now())
	require.ErrorIs(t, err, ErrInvalidOperation)
}
