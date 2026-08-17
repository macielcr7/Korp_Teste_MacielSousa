// Package stockcommand defines persistence ports for stock debit commands.
package stockcommand

import (
	"context"

	commandentity "github.com/macielcr7/korp-teste/services/inventory/internal/domain/entity/inventory/stockcommand"
)

// CommitResult describes the result of an idempotent commit attempt.
type CommitResult struct {
	Command  commandentity.StockCommand
	Replayed bool
}

// Committer atomically and idempotently applies a stock debit command.
type Committer interface {
	Commit(ctx context.Context, command commandentity.StockCommand) (CommitResult, error)
}

// Finder retrieves a previously processed stock command.
type Finder interface {
	GetByID(ctx context.Context, commandID string) (commandentity.StockCommand, error)
}
