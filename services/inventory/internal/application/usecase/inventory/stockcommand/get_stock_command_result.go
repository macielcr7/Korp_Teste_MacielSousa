package stockcommand

import (
	"context"
	"strings"

	commandentity "github.com/macielcr7/korp-teste/services/inventory/internal/domain/entity/inventory/stockcommand"
	commandrepo "github.com/macielcr7/korp-teste/services/inventory/internal/domain/repository/inventory/stockcommand"
)

// GetStockCommandResult retrieves the durable result of a stock command.
type GetStockCommandResult struct {
	repository commandrepo.Finder
}

// NewGetStockCommandResult builds the command-result query.
func NewGetStockCommandResult(repository commandrepo.Finder) *GetStockCommandResult {
	return &GetStockCommandResult{repository: repository}
}

// Execute retrieves one stock command result.
func (useCase *GetStockCommandResult) Execute(ctx context.Context, commandID string) (commandentity.StockCommand, error) {
	if strings.TrimSpace(commandID) == "" {
		return commandentity.StockCommand{}, commandentity.ErrInvalidCommand
	}
	return useCase.repository.GetByID(ctx, commandID)
}
