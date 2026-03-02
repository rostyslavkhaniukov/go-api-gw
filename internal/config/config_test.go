package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeConfig(t *testing.T, data string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(data), 0o644))
	return path
}

func TestLoad_Defaults(t *testing.T) {
	cfg, err := Load(writeConfig(t, "{}"))
	require.NoError(t, err)

	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, 9090, cfg.Server.MetricsPort)
	assert.Equal(t, 10*time.Second, cfg.Server.ReadTimeout)
	assert.Equal(t, 15*time.Second, cfg.Server.WriteTimeout)
	assert.Equal(t, 10*time.Second, cfg.Server.ShutdownTimeout)
	assert.Equal(t, "localhost:6379", cfg.Redis.Addr)
	assert.Empty(t, cfg.Redis.Password)
	assert.Equal(t, 0, cfg.Redis.DB)
	assert.Equal(t, 60*time.Second, cfg.RateLimit.Window)
	assert.Empty(t, cfg.Upstreams)
}

func TestLoad_Full(t *testing.T) {
	yamlData := `
server:
  port: 3000
  read_timeout: 5s
  write_timeout: 20s
  shutdown_timeout: 30s

redis:
  addr: redis:6380
  password: secret
  db: 2

upstreams:
  - prefix: /api/v1/users
    url: http://upstream-1:8081
    strip_prefix: false

rate_limit:
  window: 120s
`
	cfg, err := Load(writeConfig(t, yamlData))
	require.NoError(t, err)

	assert.Equal(t, 3000, cfg.Server.Port)
	assert.Equal(t, 5*time.Second, cfg.Server.ReadTimeout)
	assert.Equal(t, 20*time.Second, cfg.Server.WriteTimeout)
	assert.Equal(t, 30*time.Second, cfg.Server.ShutdownTimeout)
	assert.Equal(t, "redis:6380", cfg.Redis.Addr)
	assert.Equal(t, "secret", cfg.Redis.Password)
	assert.Equal(t, 2, cfg.Redis.DB)
	assert.Equal(t, 120*time.Second, cfg.RateLimit.Window)
	require.Len(t, cfg.Upstreams, 1)
	assert.Equal(t, "/api/v1/users", cfg.Upstreams[0].Prefix)
	assert.Equal(t, "http://upstream-1:8081", cfg.Upstreams[0].URL)
}

func TestLoad_Upstreams(t *testing.T) {
	yamlData := `
upstreams:
  - prefix: /api/v1/users
    url: http://upstream-1:8081
    strip_prefix: false
  - prefix: /api/v1/products
    url: http://upstream-2:8082
    strip_prefix: true
`
	cfg, err := Load(writeConfig(t, yamlData))
	require.NoError(t, err)

	require.Len(t, cfg.Upstreams, 2)
	assert.Equal(t, "/api/v1/users", cfg.Upstreams[0].Prefix)
	assert.Equal(t, "http://upstream-1:8081", cfg.Upstreams[0].URL)
	assert.False(t, cfg.Upstreams[0].StripPrefix)
	assert.Equal(t, "/api/v1/products", cfg.Upstreams[1].Prefix)
	assert.Equal(t, "http://upstream-2:8082", cfg.Upstreams[1].URL)
	assert.True(t, cfg.Upstreams[1].StripPrefix)
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	assert.Error(t, err)
}

func TestLoad_InvalidYAML(t *testing.T) {
	_, err := Load(writeConfig(t, "{{invalid yaml:::"))
	assert.Error(t, err)
}

func TestLoad_EmptyFile(t *testing.T) {
	cfg, err := Load(writeConfig(t, ""))
	require.NoError(t, err)

	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, "localhost:6379", cfg.Redis.Addr)
	assert.Equal(t, 60*time.Second, cfg.RateLimit.Window)
}

func TestLoad_PartialConfig(t *testing.T) {
	yamlData := `
server:
  port: 3000
redis:
  addr: redis:6380
`
	cfg, err := Load(writeConfig(t, yamlData))
	require.NoError(t, err)

	// Explicitly set values
	assert.Equal(t, 3000, cfg.Server.Port)
	assert.Equal(t, "redis:6380", cfg.Redis.Addr)

	// Defaults for unset values
	assert.Equal(t, 10*time.Second, cfg.Server.ReadTimeout)
	assert.Equal(t, 15*time.Second, cfg.Server.WriteTimeout)
	assert.Empty(t, cfg.Redis.Password)
	assert.Equal(t, 0, cfg.Redis.DB)
	assert.Equal(t, 60*time.Second, cfg.RateLimit.Window)
	assert.Empty(t, cfg.Upstreams)
}

func TestLoad_ZeroPort(t *testing.T) {
	yamlData := `
server:
  port: 0
`
	cfg, err := Load(writeConfig(t, yamlData))
	require.NoError(t, err)

	assert.Equal(t, 0, cfg.Server.Port)
}

func TestLoad_StripPrefixDefault(t *testing.T) {
	yamlData := `
upstreams:
  - prefix: /api/v1/users
    url: http://upstream-1:8081
`
	cfg, err := Load(writeConfig(t, yamlData))
	require.NoError(t, err)

	require.Len(t, cfg.Upstreams, 1)
	assert.False(t, cfg.Upstreams[0].StripPrefix)
}

func TestLoad_EnvVarExpansion(t *testing.T) {
	t.Setenv("TEST_REDIS_ADDR", "redis-host:6380")
	t.Setenv("TEST_REDIS_DB", "3")

	yamlData := `
redis:
  addr: ${TEST_REDIS_ADDR}
  db: ${TEST_REDIS_DB}
`
	cfg, err := Load(writeConfig(t, yamlData))
	require.NoError(t, err)

	assert.Equal(t, "redis-host:6380", cfg.Redis.Addr)
	assert.Equal(t, 3, cfg.Redis.DB)
}

func TestLoad_EnvVarWithDefault(t *testing.T) {
	// Ensure the var is NOT set.
	require.NoError(t, os.Unsetenv("TEST_UNSET_VAR"))

	yamlData := `
redis:
  addr: ${TEST_UNSET_VAR:-fallback:6379}
`
	cfg, err := Load(writeConfig(t, yamlData))
	require.NoError(t, err)

	assert.Equal(t, "fallback:6379", cfg.Redis.Addr)
}

func TestLoad_EnvVarDefaultOverriddenWhenSet(t *testing.T) {
	t.Setenv("TEST_OVERRIDE_VAR", "actual:6380")

	yamlData := `
redis:
  addr: ${TEST_OVERRIDE_VAR:-fallback:6379}
`
	cfg, err := Load(writeConfig(t, yamlData))
	require.NoError(t, err)

	assert.Equal(t, "actual:6380", cfg.Redis.Addr)
}

func TestLoad_TokenCacheDefault(t *testing.T) {
	cfg, err := Load(writeConfig(t, "{}"))
	require.NoError(t, err)

	assert.False(t, cfg.TokenCache.Enabled)
	assert.Equal(t, time.Duration(0), cfg.TokenCache.TTL)
}

func TestLoad_TokenCacheParsed(t *testing.T) {
	yamlData := `
token_cache:
  enabled: true
  ttl: 5s
`
	cfg, err := Load(writeConfig(t, yamlData))
	require.NoError(t, err)

	assert.True(t, cfg.TokenCache.Enabled)
	assert.Equal(t, 5*time.Second, cfg.TokenCache.TTL)
}

func TestValidate(t *testing.T) {
	validCfg := func() *Config {
		return &Config{
			Server:         ServerConfig{Port: 8080, MetricsPort: 9090},
			Upstreams:      []UpstreamConfig{{Prefix: "/api", URL: "http://localhost:8080"}},
			RateLimit:      RateLimitConfig{Window: 60 * time.Second},
			CircuitBreaker: CircuitBreakerConfig{MaxFailures: 5, Timeout: 60 * time.Second},
		}
	}

	tests := []struct {
		name    string
		modify  func(*Config)
		wantErr bool
	}{
		{
			name:   "valid",
			modify: func(_ *Config) {},
		},
		{
			name:    "zero port",
			modify:  func(c *Config) { c.Server.Port = 0 },
			wantErr: true,
		},
		{
			name:    "port too high",
			modify:  func(c *Config) { c.Server.Port = 70000 },
			wantErr: true,
		},
		{
			name:    "metrics port same as port",
			modify:  func(c *Config) { c.Server.MetricsPort = 8080 },
			wantErr: true,
		},
		{
			name:    "no upstreams",
			modify:  func(c *Config) { c.Upstreams = nil },
			wantErr: true,
		},
		{
			name:    "zero window",
			modify:  func(c *Config) { c.RateLimit.Window = 0 },
			wantErr: true,
		},
		{
			name:    "sub-second window",
			modify:  func(c *Config) { c.RateLimit.Window = 500 * time.Millisecond },
			wantErr: true,
		},
		{
			name:    "invalid URL",
			modify:  func(c *Config) { c.Upstreams[0].URL = "not-a-url" },
			wantErr: true,
		},
		{
			name:    "non-HTTP scheme",
			modify:  func(c *Config) { c.Upstreams[0].URL = "ftp://localhost:21" },
			wantErr: true,
		},
		{
			name:    "token cache enabled with zero TTL",
			modify:  func(c *Config) { c.TokenCache = TokenCacheConfig{Enabled: true, TTL: 0} },
			wantErr: true,
		},
		{
			name:    "zero max_failures",
			modify:  func(c *Config) { c.CircuitBreaker.MaxFailures = 0 },
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validCfg()
			tt.modify(cfg)
			if tt.wantErr {
				assert.Error(t, cfg.Validate())
			} else {
				assert.NoError(t, cfg.Validate())
			}
		})
	}
}
