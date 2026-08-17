package prometheus

import (
	"context"
	"testing"
	"time"

	promclient "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	commandentity "github.com/macielcr7/korp-teste/services/inventory/internal/domain/entity/inventory/stockcommand"
	commandrepo "github.com/macielcr7/korp-teste/services/inventory/internal/domain/repository/inventory/stockcommand"
)

type committerStub struct {
	result commandrepo.CommitResult
	err    error
}

func (stub committerStub) Commit(context.Context, commandentity.StockCommand) (commandrepo.CommitResult, error) {
	return stub.result, stub.err
}

func TestStockCommandCommitterRecordsCommittedDebit(t *testing.T) {
	metrics := newTestMetrics()
	committer := NewStockCommandCommitter(committerStub{result: commandrepo.CommitResult{
		Command: committedCommand(t),
	}}, metrics)

	_, err := committer.Commit(context.Background(), commandentity.StockCommand{})

	require.NoError(t, err)
	assert.Equal(t, float64(1), testutil.ToFloat64(metrics.stockDebitRuns.WithLabelValues("committed")))
}

func TestStockCommandCommitterRecordsInsufficientStock(t *testing.T) {
	metrics := newTestMetrics()
	committer := NewStockCommandCommitter(committerStub{err: commandentity.ErrInsufficientStock}, metrics)

	_, err := committer.Commit(context.Background(), commandentity.StockCommand{})

	assert.ErrorIs(t, err, commandentity.ErrInsufficientStock)
	assert.Equal(t, float64(1), testutil.ToFloat64(metrics.stockDebitRuns.WithLabelValues("insufficient_stock")))
}

func TestStockCommandCommitterPrioritizesIdempotentReplay(t *testing.T) {
	metrics := newTestMetrics()
	committer := NewStockCommandCommitter(committerStub{result: commandrepo.CommitResult{
		Command: committedCommand(t), Replayed: true,
	}}, metrics)

	_, err := committer.Commit(context.Background(), commandentity.StockCommand{})

	require.NoError(t, err)
	assert.Equal(t, float64(1), testutil.ToFloat64(metrics.stockDebitRuns.WithLabelValues("idempotent_replay")))
	assert.Equal(t, float64(0), testutil.ToFloat64(metrics.stockDebitRuns.WithLabelValues("committed")))
}

func committedCommand(t *testing.T) commandentity.StockCommand {
	t.Helper()
	command, err := commandentity.Rehydrate(commandentity.Snapshot{
		CommandID: "command", InvoiceID: "invoice", PayloadHash: "hash", Status: commandentity.StatusCommitted,
		Items: []commandentity.Item{{ProductID: "product", Quantity: 1}}, CreatedAt: time.Now(),
	})
	require.NoError(t, err)
	return command
}

func newTestMetrics() *Metrics {
	registry := promclient.NewRegistry()
	return New(registry, registry)
}
