package gateway

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/errgroup"

	"api-gw/internal/config"
	"api-gw/internal/health"
	"api-gw/internal/proxy"
	"api-gw/internal/ratelimiter"
	internalredis "api-gw/internal/redis"
	"api-gw/internal/token"
)

// redisChecker adapts *redis.Client to the health.Checker interface.
type redisChecker struct {
	client *redis.Client
}

func (r redisChecker) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// Run starts the API gateway with the given configuration.
// It blocks until the context is cancelled or the server fails.
func Run(ctx context.Context, cfg *config.Config) error {
	redisClient, err := internalredis.NewClient(ctx, cfg.Redis)
	if err != nil {
		return fmt.Errorf("connect redis: %w", err)
	}
	defer redisClient.Close()

	var tokenStore token.Store = token.NewRedisStore(redisClient)
	if cfg.TokenCache.Enabled {
		tokenStore = token.NewCachedStore(ctx, tokenStore, cfg.TokenCache.TTL)
		slog.InfoContext(ctx, "token cache enabled", "ttl", cfg.TokenCache.TTL)
	}
	limiter := ratelimiter.NewRedisLimiter(redisClient, cfg.RateLimit.Window)

	entries, err := proxy.BuildUpstreams(cfg.Upstreams, cfg.Proxy, cfg.CircuitBreaker)
	if err != nil {
		return fmt.Errorf("build upstreams: %w", err)
	}
	for _, e := range entries {
		slog.InfoContext(ctx, "upstream registered", "prefix", e.Prefix, "strip_prefix", e.StripPrefix)
	}

	router := NewRouter(tokenStore, limiter, proxy.NewDispatcher(entries), cfg.Server.MaxBodyBytes)

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:           router,
		ReadTimeout:       cfg.Server.ReadTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
	}

	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.Handler())
	metricsMux.Handle("/healthz", health.Healthz())
	metricsMux.Handle("/readyz", health.Readyz(redisChecker{redisClient}))
	metricsSrv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Server.MetricsPort),
		Handler:           metricsMux,
		ReadTimeout:       cfg.Server.ReadTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
	}

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		slog.Info("server starting", "port", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})

	g.Go(func() error {
		slog.Info("metrics server starting", "port", cfg.Server.MetricsPort)
		if err := metricsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})

	g.Go(func() error {
		<-gCtx.Done()
		slog.Info("shutting down")

		// Shut down both servers concurrently so neither wastes the
		// other's timeout budget.
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
		defer shutdownCancel()

		var sg errgroup.Group
		sg.Go(func() error { return metricsSrv.Shutdown(shutdownCtx) })
		sg.Go(func() error { return srv.Shutdown(shutdownCtx) })
		return sg.Wait()
	})

	return g.Wait()
}
