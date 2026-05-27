package postgresql

import (
	"context"
	"fmt"
	"net"
	"restaurants/internal/config"
	repeatable "restaurants/pkg/utils"
	"time"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

type Client interface {
	Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
}

func NewClient(ctx context.Context, maxAttempts int, sc config.StorageConfig) (pool *pgxpool.Pool, err error) {
	dsn := fmt.Sprintf("postgresql://%s:%s@%s:%s/%s?sslmode=disable", sc.Username, sc.Password, sc.Host, sc.Port, sc.Database)

	// err = migrations.RunMigrations("db/migrations", dsn)
	// if err != nil {
	// 	log.Fatalf("Migration failed: %v", err)
	// }
	// log.Println("Migrations completed successfully")
	err = repeatable.DoWithTries(func() error {
		cfg, err := pgxpool.ParseConfig(dsn)

		if err != nil {
			fmt.Println("failed to parse pg config: %w", err)
			fmt.Println("error connection database")
			return err
		}

		cfg.MaxConns = int32(sc.PgPoolMaxConn)
		cfg.HealthCheckPeriod = 2 * time.Minute
		cfg.MaxConnLifetime = 30 * time.Minute
		cfg.MaxConnIdleTime = 20 * time.Minute

		cfg.ConnConfig.ConnectTimeout = 3 * time.Minute
		cfg.ConnConfig.DialFunc = (&net.Dialer{
			KeepAlive: cfg.HealthCheckPeriod,
			Timeout:   cfg.ConnConfig.ConnectTimeout,
		}).DialContext
		pool, err = pgxpool.ConnectConfig(ctx, cfg)

		if err != nil {
			return err
		}

		return nil
	}, maxAttempts, 5*time.Second)

	if err != nil {
		fmt.Println(err, "error do with tries postgresql")
		return nil, err
	}
	return pool, nil
}
