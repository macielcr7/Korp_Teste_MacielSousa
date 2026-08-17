package closureoperation

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/macielcr7/korp-teste/services/billing/internal/application/service/shared"
	operationentity "github.com/macielcr7/korp-teste/services/billing/internal/domain/entity/billing/closureoperation"
	operationrepository "github.com/macielcr7/korp-teste/services/billing/internal/domain/repository/billing/closureoperation"
)

var ErrIdempotencyKeyRequired = errors.New("Idempotency-Key header is required")

// Request creates or returns an idempotent closure operation.
type Request struct {
	repository operationrepository.Creator
	ids        shared.IDGenerator
	now        func() time.Time
}

func NewRequest(repository operationrepository.Creator, ids shared.IDGenerator, now func() time.Time) *Request {
	return &Request{repository: repository, ids: ids, now: now}
}

func (useCase *Request) Execute(ctx context.Context, invoiceID, idempotencyKey string) (operationentity.Operation, bool, error) {
	if strings.TrimSpace(idempotencyKey) == "" {
		return operationentity.Operation{}, false, ErrIdempotencyKeyRequired
	}
	operation, err := operationentity.New(useCase.ids.NewID(), invoiceID, idempotencyKey, useCase.ids.NewID(), useCase.now())
	if err != nil {
		return operationentity.Operation{}, false, err
	}
	return useCase.repository.CreateOrGet(ctx, operation)
}
