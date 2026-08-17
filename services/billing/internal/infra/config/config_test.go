package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoadRequiresInventoryInternalToken(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://billing")
	t.Setenv("INVENTORY_INTERNAL_TOKEN", "")

	_, err := Load()

	require.ErrorContains(t, err, "INVENTORY_INTERNAL_TOKEN")
}

func TestLoadRequiresLeaseLongerThanDownstreamTimeout(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://billing")
	t.Setenv("INVENTORY_INTERNAL_TOKEN", "internal-secret")
	t.Setenv("INVENTORY_HTTP_TIMEOUT", "10s")
	t.Setenv("WORKER_LEASE_DURATION", "10s")

	_, err := Load()

	require.ErrorContains(t, err, "WORKER_LEASE_DURATION")
}

func TestLoadAcceptsSafeLeaseAndTimeout(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://billing")
	t.Setenv("INVENTORY_INTERNAL_TOKEN", "internal-secret")
	t.Setenv("INVENTORY_HTTP_TIMEOUT", "5s")
	t.Setenv("WORKER_LEASE_DURATION", "30s")

	configuration, err := Load()

	require.NoError(t, err)
	require.Equal(t, "http://inventory:8083", configuration.InventoryBaseURL)
	require.Equal(t, 5*time.Second, configuration.InventoryHTTPTimeout)
	require.Equal(t, 30*time.Second, configuration.WorkerLeaseDuration)
}
