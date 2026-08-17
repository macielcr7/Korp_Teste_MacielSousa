package invoice

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	invoiceentity "github.com/macielcr7/korp-teste/services/billing/internal/domain/entity/billing/invoice"
	invoicerepository "github.com/macielcr7/korp-teste/services/billing/internal/domain/repository/billing/invoice"
)

type listRepositoryFake struct {
	criteria invoicerepository.ListCriteria
	result   invoicerepository.ListResult
}

func (fake *listRepositoryFake) Create(context.Context, invoiceentity.Invoice) (invoiceentity.Invoice, error) {
	panic("not used")
}

func (fake *listRepositoryFake) FindByID(context.Context, string) (invoiceentity.Invoice, error) {
	panic("not used")
}

func (fake *listRepositoryFake) List(_ context.Context, criteria invoicerepository.ListCriteria) (invoicerepository.ListResult, error) {
	fake.criteria = criteria
	return fake.result, nil
}

func TestListAppliesStatusAndPagination(t *testing.T) {
	repository := &listRepositoryFake{result: invoicerepository.ListResult{Total: 21}}

	output, err := NewList(repository).Execute(context.Background(), ListInput{
		Status: invoiceentity.StatusClosed, Limit: 10, Offset: 20,
	})

	require.NoError(t, err)
	require.NotNil(t, repository.criteria.Status)
	require.Equal(t, invoiceentity.StatusClosed, *repository.criteria.Status)
	require.Equal(t, 10, repository.criteria.Limit)
	require.Equal(t, 20, repository.criteria.Offset)
	require.Equal(t, int64(21), output.Total)
}

func TestListRejectsInvalidStatus(t *testing.T) {
	_, err := NewList(&listRepositoryFake{}).Execute(context.Background(), ListInput{Status: "INVALID"})

	require.ErrorIs(t, err, ErrInvalidListStatus)
}
