package invoice

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	operationentity "github.com/macielcr7/korp-teste/services/billing/internal/domain/entity/billing/closureoperation"
	invoiceentity "github.com/macielcr7/korp-teste/services/billing/internal/domain/entity/billing/invoice"
	invoicerepository "github.com/macielcr7/korp-teste/services/billing/internal/domain/repository/billing/invoice"
)

type invoiceDetailFinderFake struct {
	detail invoicerepository.Detail
	err    error
}

func (repository *invoiceDetailFinderFake) FindDetail(context.Context, string) (invoicerepository.Detail, error) {
	return repository.detail, repository.err
}

func TestGetDetailReturnsActiveOperationForPollingResume(t *testing.T) {
	for _, status := range []operationentity.Status{operationentity.StatusPending, operationentity.StatusRetrying} {
		t.Run(string(status), func(t *testing.T) {
			inv := invoiceFixture(t)
			useCase := NewGetDetail(&invoiceDetailFinderFake{detail: invoicerepository.Detail{
				Invoice: inv,
				ActiveClosureOperation: &invoicerepository.ActiveClosureOperation{
					OperationID: "operation-1",
					Status:      status,
				},
			}})

			detail, err := useCase.Execute(context.Background(), inv.ID())

			require.NoError(t, err)
			require.NotNil(t, detail.ActiveClosureOperation)
			require.Equal(t, "operation-1", detail.ActiveClosureOperation.OperationID)
			require.Equal(t, status, detail.ActiveClosureOperation.Status)
		})
	}
}

func TestGetDetailReturnsNullWhenNoOperationIsActive(t *testing.T) {
	inv := invoiceFixture(t)
	useCase := NewGetDetail(&invoiceDetailFinderFake{detail: invoicerepository.Detail{Invoice: inv}})

	detail, err := useCase.Execute(context.Background(), inv.ID())

	require.NoError(t, err)
	require.Nil(t, detail.ActiveClosureOperation)
}

func invoiceFixture(t *testing.T) invoiceentity.Invoice {
	t.Helper()
	item, err := invoiceentity.NewItem("product-1", "SKU-1", "Keyboard", 1)
	require.NoError(t, err)
	inv, err := invoiceentity.Rehydrate("invoice-1", 1, invoiceentity.StatusOpen, []invoiceentity.Item{item}, 1, time.Now(), nil)
	require.NoError(t, err)
	return inv
}
