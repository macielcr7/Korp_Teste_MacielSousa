package prometheus

import (
	"context"
	"errors"

	commandentity "github.com/macielcr7/korp-teste/services/inventory/internal/domain/entity/inventory/stockcommand"
	commandrepo "github.com/macielcr7/korp-teste/services/inventory/internal/domain/repository/inventory/stockcommand"
)

// StockCommandCommitter decorates stock commits with business-result metrics.
type StockCommandCommitter struct {
	delegate commandrepo.Committer
	metrics  *Metrics
}

// NewStockCommandCommitter creates an instrumented stock-command committer.
func NewStockCommandCommitter(delegate commandrepo.Committer, metrics *Metrics) *StockCommandCommitter {
	return &StockCommandCommitter{delegate: delegate, metrics: metrics}
}

// Commit delegates the atomic operation and records its result.
func (committer *StockCommandCommitter) Commit(
	ctx context.Context,
	command commandentity.StockCommand,
) (commandrepo.CommitResult, error) {
	result, err := committer.delegate.Commit(ctx, command)
	committer.metrics.observeStockDebit(stockDebitResult(result, err))
	return result, err
}

func stockDebitResult(result commandrepo.CommitResult, err error) string {
	if result.Replayed {
		return "idempotent_replay"
	}
	switch {
	case err == nil && result.Command.Status() == commandentity.StatusCommitted:
		return "committed"
	case errors.Is(err, commandentity.ErrInsufficientStock):
		return "insufficient_stock"
	case errors.Is(err, commandentity.ErrProductNotFound):
		return "product_not_found"
	case errors.Is(err, commandentity.ErrIdempotencyConflict):
		return "idempotency_conflict"
	case errors.Is(err, commandentity.ErrInvalidCommand),
		errors.Is(err, commandentity.ErrInvalidQuantity),
		errors.Is(err, commandentity.ErrDuplicateProduct):
		return "invalid_command"
	default:
		return "failed"
	}
}
