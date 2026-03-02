package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"api-gw/internal/config"
	internalredis "api-gw/internal/redis"
	"api-gw/internal/token"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		return errors.New("usage: seed --config <config.yaml> <tokens.json>")
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}

	ctx := context.Background()
	redisClient, err := internalredis.NewClient(ctx, cfg.Redis)
	if err != nil {
		return fmt.Errorf("connect redis: %w", err)
	}
	defer redisClient.Close()

	data, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	var tokens []token.Token
	if err := json.Unmarshal(data, &tokens); err != nil {
		return fmt.Errorf("parse tokens: %w", err)
	}

	store := token.NewRedisStore(redisClient)

	for i := range tokens {
		masked := tokens[i].APIKey
		if len(masked) > 8 {
			masked = masked[:8] + "***"
		}
		if err := store.Set(ctx, &tokens[i]); err != nil {
			return fmt.Errorf("store token %q: %w", masked, err)
		}
		slog.Info("seeded token", "api_key", masked)
	}

	slog.Info("done", "count", len(tokens))
	return nil
}
