// Package stockcommand contains inventory stock-command use cases.
package stockcommand

import (
	"context"

	commandentity "github.com/macielcr7/korp-teste/services/inventory/internal/domain/entity/inventory/stockcommand"
	commandrepo "github.com/macielcr7/korp-teste/services/inventory/internal/domain/repository/inventory/stockcommand"
)

// CommitStockDebitInput contains an idempotent stock debit request.
type CommitStockDebitInput struct {
	CommandID string
	InvoiceID string
	Items     []commandentity.Item
}

// CommitStockDebit atomically applies all requested product debits.
type CommitStockDebit struct {
	repository commandrepo.Committer
}

// NewCommitStockDebit builds the commit-stock-debit use case.
func NewCommitStockDebit(repository commandrepo.Committer) *CommitStockDebit {
	return &CommitStockDebit{repository: repository}
}

// Execute validates and commits an idempotent stock debit.
func (useCase *CommitStockDebit) Execute(ctx context.Context, input CommitStockDebitInput) (commandentity.StockCommand, error) {
	command, err := commandentity.New(input.CommandID, input.InvoiceID, input.Items)
	if err != nil {
		return commandentity.StockCommand{}, err
	}

	result, err := useCase.repository.Commit(ctx, command)
	return result.Command, err
}
