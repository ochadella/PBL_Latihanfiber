package database

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"siakad-mini/config"
)

// NewPool membuat connection pool ke PostgreSQL berdasarkan isi .env.
func NewPool(ctx context.Context) (*pgxpool.Pool, error) {
	dsn := url.URL{
		Scheme: "postgres",
		User: url.UserPassword(
			config.GetEnv("DB_USER", "postgres"),
			config.GetEnv("DB_PASSWORD", ""),
		),
		Host:     config.GetEnv("DB_HOST", "localhost") + ":" + config.GetEnv("DB_PORT", "5432"),
		Path:     config.GetEnv("DB_NAME", "siakad_mini"),
		RawQuery: "sslmode=" + config.GetEnv("DB_SSLMODE", "disable"),
	}

	poolConfig, err := pgxpool.ParseConfig(dsn.String())
	if err != nil {
		return nil, fmt.Errorf("konfigurasi database tidak valid: %w", err)
	}
	poolConfig.MaxConns = 10
	poolConfig.MinConns = 2
	poolConfig.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("gagal membuat pool: %w", err)
	}

	// Pastikan database benar-benar bisa dihubungi.
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database tidak dapat dihubungi: %w", err)
	}

	return pool, nil
}
