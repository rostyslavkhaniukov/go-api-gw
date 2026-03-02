package redis

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"api-gw/internal/config"
)

func TestNewClient_Success(t *testing.T) {
	mr := miniredis.RunT(t)

	client, err := NewClient(context.Background(), config.RedisConfig{
		Addr:         mr.Addr(),
		PoolSize:     5,
		MinIdleConns: 1,
	})
	require.NoError(t, err)
	defer client.Close()

	assert.NoError(t, client.Ping(context.Background()).Err())
}

func TestNewClient_WithPassword(t *testing.T) {
	mr := miniredis.RunT(t)
	mr.RequireAuth("secret")

	client, err := NewClient(context.Background(), config.RedisConfig{
		Addr:     mr.Addr(),
		Password: "secret",
	})
	require.NoError(t, err)
	defer client.Close()

	assert.NoError(t, client.Ping(context.Background()).Err())
}

func TestNewClient_BadPassword(t *testing.T) {
	mr := miniredis.RunT(t)
	mr.RequireAuth("secret")

	_, err := NewClient(context.Background(), config.RedisConfig{
		Addr:     mr.Addr(),
		Password: "wrong",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "redis ping")
}

func TestNewClient_Unreachable(t *testing.T) {
	_, err := NewClient(context.Background(), config.RedisConfig{
		Addr: "localhost:0",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "redis ping")
}

func TestNewClient_TLS(t *testing.T) {
	// With TLS enabled against a non-TLS miniredis, the connection should fail.
	mr := miniredis.RunT(t)

	_, err := NewClient(context.Background(), config.RedisConfig{
		Addr:       mr.Addr(),
		TLSEnabled: true,
	})
	require.Error(t, err)
}
