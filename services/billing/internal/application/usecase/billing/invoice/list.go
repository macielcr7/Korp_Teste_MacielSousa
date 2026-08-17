package invoice

import (
	"context"
	"errors"
	"fmt"

	invoiceentity "github.com/macielcr7/korp-teste/services/billing/internal/domain/entity/billing/invoice"
	invoicerepository "github.com/macielcr7/korp-teste/services/billing/internal/domain/repository/billing/invoice"
)

const (
	defaultListLimit = 50
	maximumListLimit = 100
)

// ErrInvalidListStatus indicates an unsupported invoice status filter.
var ErrInvalidListStatus = errors.New("invoice status filter must be OPEN or CLOSED")

// ListInput contains remote invoice filters and pagination.
type ListInput struct {
	Status invoiceentity.Status
	Limit  int
	Offset int
}

// ListOutput contains a filtered invoice page and its total.
type ListOutput struct {
	Items  []invoiceentity.Invoice
	Total  int64
	Limit  int
	Offset int
}

// List retrieves invoices ordered by number descending.
type List struct{ repository invoicerepository.Lister }

func NewList(repository invoicerepository.Lister) *List { return &List{repository: repository} }

func (useCase *List) Execute(ctx context.Context, input ListInput) (ListOutput, error) {
	if input.Status != "" && input.Status != invoiceentity.StatusOpen && input.Status != invoiceentity.StatusClosed {
		return ListOutput{}, ErrInvalidListStatus
	}
	if input.Limit <= 0 {
		input.Limit = defaultListLimit
	}
	if input.Limit > maximumListLimit {
		input.Limit = maximumListLimit
	}
	if input.Offset < 0 {
		input.Offset = 0
	}

	criteria := invoicerepository.ListCriteria{Limit: input.Limit, Offset: input.Offset}
	if input.Status != "" {
		criteria.Status = &input.Status
	}
	result, err := useCase.repository.List(ctx, criteria)
	if err != nil {
		return ListOutput{}, fmt.Errorf("list invoices: %w", err)
	}
	return ListOutput{Items: result.Items, Total: result.Total, Limit: input.Limit, Offset: input.Offset}, nil
}
