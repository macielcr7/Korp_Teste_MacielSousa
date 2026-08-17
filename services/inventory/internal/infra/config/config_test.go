package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadRequiresAndTrimsInternalToken(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://inventory:test@localhost/inventory")
	t.Setenv("INVENTORY_INTERNAL_TOKEN", "  internal-secret  ")

	loaded, err := Load()

	require.NoError(t, err)
	assert.Equal(t, "internal-secret", loaded.InternalAPIToken)
}

func TestLoadRejectsMissingInternalToken(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://inventory:test@localhost/inventory")
	t.Setenv("INVENTORY_INTERNAL_TOKEN", " ")

	_, err := Load()

	assert.EqualError(t, err, "INVENTORY_INTERNAL_TOKEN is required")
}
