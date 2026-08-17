package product

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	now := time.Date(2026, 8, 12, 10, 0, 0, 0, time.FixedZone("BRT", -3*60*60))

	got, err := New("product-id", " sku-1 ", " Product one ", 10, now)

	require.NoError(t, err)
	assert.Equal(t, "SKU-1", got.Code())
	assert.Equal(t, "Product one", got.Description())
	assert.Equal(t, int64(10), got.Balance())
	assert.Equal(t, int64(1), got.Version())
	assert.Equal(t, now.UTC(), got.CreatedAt())
}

func TestNewCountsUnicodeCharactersInsteadOfBytes(t *testing.T) {
	description := strings.Repeat("ç", 255)

	got, err := New("product-id", "sku-1", description, 1, time.Now())

	require.NoError(t, err)
	assert.Equal(t, "SKU-1", got.Code())
	assert.Equal(t, description, got.Description())

	_, err = New("product-id", "SKU", strings.Repeat("ç", 256), 1, time.Now())
	assert.ErrorIs(t, err, ErrInvalidDescription)
}

func TestNewRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name        string
		id          string
		code        string
		description string
		balance     int64
		wantErr     error
	}{
		{name: "empty id", code: "SKU", description: "Product", wantErr: ErrInvalidID},
		{name: "empty code", id: "id", description: "Product", wantErr: ErrInvalidCode},
		{name: "code with unsupported characters", id: "id", code: "SKU 01", description: "Product", wantErr: ErrInvalidCode},
		{name: "oversized code", id: "id", code: strings.Repeat("A", 65), description: "Product", wantErr: ErrInvalidCode},
		{name: "empty description", id: "id", code: "SKU", wantErr: ErrInvalidDescription},
		{name: "negative balance", id: "id", code: "SKU", description: "Product", balance: -1, wantErr: ErrInvalidBalance},
		{name: "unsafe balance", id: "id", code: "SKU", description: "Product", balance: MaximumSafeInteger + 1, wantErr: ErrInvalidBalance},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.id, tt.code, tt.description, tt.balance, time.Now())
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}
