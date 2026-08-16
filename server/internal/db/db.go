package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	dbgen "github.com/esuEdu/go-tauri-discord/internal/db/gen"
)

type Pool struct {
	*dbgen.Queries
	pool *pgxpool.Pool
}

func Connect(ctx context.Context, url string) (*Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	cfg.MaxConns = 20
	cfg.MinConns = 2
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 30 * time.Minute
	cfg.HealthCheckPeriod = time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}

	return &Pool{Queries: dbgen.New(pool), pool: pool}, nil
}

func (p *Pool) Close() { p.pool.Close() }

func (p *Pool) Raw() *pgxpool.Pool { return p.pool }

func (p *Pool) InTx(ctx context.Context, fn func(q *dbgen.Queries) error) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() {

		_ = tx.Rollback(ctx)
	}()

	if err := fn(p.Queries.WithTx(tx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func IsNoRows(err error) bool { return err == pgx.ErrNoRows }

func IsUniqueViolation(err error) bool {
	return pgErrCode(err) == "23505"
}

func IsForeignKeyViolation(err error) bool {
	return pgErrCode(err) == "23503"
}
