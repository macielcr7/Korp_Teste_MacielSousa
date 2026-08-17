package invoice

import (
	"context"
	"fmt"

	invoiceentity "github.com/macielcr7/korp-teste/services/billing/internal/domain/entity/billing/invoice"
	invoicerepository "github.com/macielcr7/korp-teste/services/billing/internal/domain/repository/billing/invoice"
)

// GetPrintable retrieves a closed invoice for rendering or printing.
type GetPrintable struct{ repository invoicerepository.Finder }

func NewGetPrintable(repository invoicerepository.Finder) *GetPrintable {
	return &GetPrintable{repository: repository}
}

func (useCase *GetPrintable) Execute(ctx context.Context, id string) (invoiceentity.Invoice, error) {
	result, err := useCase.repository.FindByID(ctx, id)
	if err != nil {
		return invoiceentity.Invoice{}, fmt.Errorf("get printable invoice: %w", err)
	}
	if result.Status() != invoiceentity.StatusClosed {
		return invoiceentity.Invoice{}, invoiceentity.ErrInvoiceNotClosed
	}
	return result, nil
}
