package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Port              int           `yaml:"port"`
	MetricsPort       int           `yaml:"metrics_port"`
	MaxBodyBytes      int64         `yaml:"max_body_bytes"`
	ReadTimeout       time.Duration `yaml:"read_timeout"`
	WriteTimeout      time.Duration `yaml:"write_timeout"`
	IdleTimeout       time.Duration `yaml:"idle_timeout"`
	ShutdownTimeout   time.Duration `yaml:"shutdown_timeout"`
	ReadHeaderTimeout time.Duration `yaml:"read_header_timeout"`
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Addr         string `yaml:"addr"`
	Password     string `yaml:"password"`
	DB           int    `yaml:"db"`
	PoolSize     int    `yaml:"pool_size"`
	MinIdleConns int    `yaml:"min_idle_conns"`
	TLSEnabled   bool   `yaml:"tls_enabled"`
}

// UpstreamConfig maps a path prefix to a backend URL.
type UpstreamConfig struct {
	Prefix      string `yaml:"prefix"`
	URL         string `yaml:"url"`
	StripPrefix bool   `yaml:"strip_prefix"`
}

// ProxyConfig holds reverse proxy transport settings.
type ProxyConfig struct {
	DialTimeout           time.Duration `yaml:"dial_timeout"`
	ResponseHeaderTimeout time.Duration `yaml:"response_header_timeout"`
	IdleConnTimeout       time.Duration `yaml:"idle_conn_timeout"`
	MaxIdleConns          int           `yaml:"max_idle_conns"`
	MaxIdleConnsPerHost   int           `yaml:"max_idle_conns_per_host"`
	MaxConnsPerHost       int           `yaml:"max_conns_per_host"`
}

// CircuitBreakerConfig holds circuit breaker settings.
type CircuitBreakerConfig struct {
	MaxFailures uint32        `yaml:"max_failures"`
	Timeout     time.Duration `yaml:"timeout"`
}

// RateLimitConfig holds rate limiting settings.
type RateLimitConfig struct {
	Window time.Duration `yaml:"window"`
}

// TokenCacheConfig holds optional in-memory token cache settings.
type TokenCacheConfig struct {
	Enabled bool          `yaml:"enabled"`
	TTL     time.Duration `yaml:"ttl"`
}

// Config holds all configuration for the API gateway.
type Config struct {
	Server         ServerConfig         `yaml:"server"`
	Redis          RedisConfig          `yaml:"redis"`
	Upstreams      []UpstreamConfig     `yaml:"upstreams"`
	Proxy          ProxyConfig          `yaml:"proxy"`
	CircuitBreaker CircuitBreakerConfig `yaml:"circuit_breaker"`
	RateLimit      RateLimitConfig      `yaml:"rate_limit"`
	TokenCache     TokenCacheConfig     `yaml:"token_cache"`
}

// Load reads YAML configuration from the given file path and applies defaults.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	cfg := &Config{
		Server: ServerConfig{
			Port:              8080,
			MetricsPort:       9090,
			MaxBodyBytes:      10 << 20, // 10 MB
			ReadTimeout:       10 * time.Second,
			WriteTimeout:      15 * time.Second,
			IdleTimeout:       120 * time.Second,
			ShutdownTimeout:   10 * time.Second,
			ReadHeaderTimeout: 5 * time.Second,
		},
		Redis: RedisConfig{
			Addr:         "localhost:6379",
			PoolSize:     128,
			MinIdleConns: 16,
		},
		Proxy: ProxyConfig{
			DialTimeout:           5 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			IdleConnTimeout:       90 * time.Second,
			MaxIdleConns:          128,
			MaxIdleConnsPerHost:   32,
			MaxConnsPerHost:       128,
		},
		CircuitBreaker: CircuitBreakerConfig{
			MaxFailures: 5,
			Timeout:     60 * time.Second,
		},
		RateLimit: RateLimitConfig{
			Window: 60 * time.Second,
		},
	}

	// Replace $$ with a placeholder before expansion so that literal
	// dollar signs survive os.Expand (e.g. passwords containing "$").
	const placeholder = "\x00DOLLAR\x00"
	raw := strings.ReplaceAll(string(data), "$$", placeholder)

	expanded := os.Expand(raw, func(key string) string {
		if name, def, ok := strings.Cut(key, ":-"); ok {
			if v, exists := os.LookupEnv(name); exists {
				return v
			}
			return def
		}
		return os.Getenv(key)
	})

	expanded = strings.ReplaceAll(expanded, placeholder, "$")

	if err := yaml.Unmarshal([]byte(expanded), cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	return cfg, nil
}

// Validate checks the configuration for invalid or missing values.
// It reports all validation errors at once rather than stopping at the first.
func (c *Config) Validate() error {
	var errs []error
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		errs = append(errs, fmt.Errorf("server port must be between 1 and 65535, got %d", c.Server.Port))
	}
	if c.Server.MetricsPort <= 0 || c.Server.MetricsPort > 65535 {
		errs = append(errs, fmt.Errorf("server metrics_port must be between 1 and 65535, got %d", c.Server.MetricsPort))
	}
	if c.Server.MetricsPort == c.Server.Port {
		errs = append(errs, fmt.Errorf("server metrics_port must differ from port, both are %d", c.Server.Port))
	}
	if len(c.Upstreams) == 0 {
		errs = append(errs, errors.New("at least one upstream must be configured"))
	}
	if c.RateLimit.Window < time.Second {
		errs = append(errs, fmt.Errorf("rate limit window must be at least 1s, got %v", c.RateLimit.Window))
	} else if c.RateLimit.Window%time.Second != 0 {
		errs = append(errs, fmt.Errorf("rate limit window must be a whole number of seconds, got %v", c.RateLimit.Window))
	}
	if c.CircuitBreaker.MaxFailures < 1 {
		errs = append(errs, fmt.Errorf("circuit_breaker.max_failures must be at least 1, got %d", c.CircuitBreaker.MaxFailures))
	}
	if c.TokenCache.Enabled && c.TokenCache.TTL <= 0 {
		errs = append(errs, errors.New("token_cache.ttl must be positive when enabled"))
	}
	for i, u := range c.Upstreams {
		if !strings.HasPrefix(u.Prefix, "/") {
			errs = append(errs, fmt.Errorf("upstream[%d] prefix must start with \"/\", got %q", i, u.Prefix))
		}
		if len(u.Prefix) > 1 && strings.HasSuffix(u.Prefix, "/") {
			errs = append(errs, fmt.Errorf("upstream[%d] prefix must not end with \"/\", got %q", i, u.Prefix))
		}
		parsed, err := url.Parse(u.URL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			errs = append(errs, fmt.Errorf("upstream[%d] has invalid URL: %q", i, u.URL))
		}
	}
	return errors.Join(errs...)
}
