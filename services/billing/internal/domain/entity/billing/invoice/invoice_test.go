package invoice

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewAndClose(t *testing.T) {
	item, err := NewItem("product-1", "SKU-1", "Keyboard", 2)
	require.NoError(t, err)
	now := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	inv, err := New("invoice-1", []Item{item}, now)
	require.NoError(t, err)
	require.Equal(t, StatusOpen, inv.Status())
	require.NoError(t, inv.Close(now.Add(time.Hour)))
	require.Equal(t, StatusClosed, inv.Status())
	require.ErrorIs(t, inv.Close(now.Add(2*time.Hour)), ErrInvoiceNotOpen)
}

func TestNewRejectsDuplicateProducts(t *testing.T) {
	item, err := NewItem("product-1", "SKU-1", "Keyboard", 1)
	require.NoError(t, err)
	_, err = New("invoice-1", []Item{item, item}, time.Now())
	require.ErrorIs(t, err, ErrDuplicateProduct)
}

func TestNewItemRejectsInvalidQuantity(t *testing.T) {
	_, err := NewItem("product-1", "SKU-1", "Keyboard", 0)
	require.ErrorIs(t, err, ErrInvalidQuantity)

	_, err = NewItem("product-1", "SKU-1", "Keyboard", MaximumSafeInteger+1)
	require.ErrorIs(t, err, ErrInvalidQuantity)
}
