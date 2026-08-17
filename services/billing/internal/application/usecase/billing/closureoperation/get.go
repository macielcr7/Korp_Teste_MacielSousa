package closureoperation

import (
	"context"
	"fmt"

	operationentity "github.com/macielcr7/korp-teste/services/billing/internal/domain/entity/billing/closureoperation"
	operationrepository "github.com/macielcr7/korp-teste/services/billing/internal/domain/repository/billing/closureoperation"
)

// Get retrieves a closure operation.
type Get struct {
	repository operationrepository.Finder
}

func NewGet(repository operationrepository.Finder) *Get { return &Get{repository: repository} }

func (useCase *Get) Execute(ctx context.Context, id string) (operationentity.Operation, error) {
	result, err := useCase.repository.FindByID(ctx, id)
	if err != nil {
		return operationentity.Operation{}, fmt.Errorf("get closure operation: %w", err)
	}
	return result, nil
}
