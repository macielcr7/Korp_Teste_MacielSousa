package invoice

import (
	"context"
	"fmt"

	operationentity "github.com/macielcr7/korp-teste/services/billing/internal/domain/entity/billing/closureoperation"
	invoiceentity "github.com/macielcr7/korp-teste/services/billing/internal/domain/entity/billing/invoice"
	invoicerepository "github.com/macielcr7/korp-teste/services/billing/internal/domain/repository/billing/invoice"
)

// ActiveClosureOperation is the polling information exposed with an invoice detail.
type ActiveClosureOperation struct {
	OperationID string
	Status      operationentity.Status
}

// Detail combines an invoice with its currently actionable closure operation.
type Detail struct {
	Invoice                invoiceentity.Invoice
	ActiveClosureOperation *ActiveClosureOperation
}

// GetDetail retrieves one invoice and its active closure operation, when present.
type GetDetail struct {
	invoices invoicerepository.DetailFinder
}

func NewGetDetail(invoices invoicerepository.DetailFinder) *GetDetail {
	return &GetDetail{invoices: invoices}
}

func (useCase *GetDetail) Execute(ctx context.Context, id string) (Detail, error) {
	detail, err := useCase.invoices.FindDetail(ctx, id)
	if err != nil {
		return Detail{}, fmt.Errorf("get invoice detail: %w", err)
	}
	result := Detail{Invoice: detail.Invoice}
	if detail.ActiveClosureOperation != nil {
		result.ActiveClosureOperation = &ActiveClosureOperation{
			OperationID: detail.ActiveClosureOperation.OperationID,
			Status:      detail.ActiveClosureOperation.Status,
		}
	}
	return result, nil
}
