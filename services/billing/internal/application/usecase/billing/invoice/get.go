package invoice

import (
	"context"
	"fmt"

	invoiceentity "github.com/macielcr7/korp-teste/services/billing/internal/domain/entity/billing/invoice"
	invoicerepository "github.com/macielcr7/korp-teste/services/billing/internal/domain/repository/billing/invoice"
)

// Get retrieves one invoice.
type Get struct{ repository invoicerepository.Finder }

func NewGet(repository invoicerepository.Finder) *Get { return &Get{repository: repository} }

func (useCase *Get) Execute(ctx context.Context, id string) (invoiceentity.Invoice, error) {
	result, err := useCase.repository.FindByID(ctx, id)
	if err != nil {
		return invoiceentity.Invoice{}, fmt.Errorf("get invoice: %w", err)
	}
	return result, nil
}
