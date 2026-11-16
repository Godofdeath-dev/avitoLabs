package transaction

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type txKey struct{}

type QueryExecutor interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Manager struct {
	pool    *pgxpool.Pool
	timeout time.Duration
}

func NewManager(pool *pgxpool.Pool, timeout time.Duration) *Manager {
	return &Manager{pool: pool, timeout: timeout}
}

func (tm *Manager) Do(ctx context.Context, fn func(ctx context.Context) error) (err error) {
	ctx, cancel := context.WithTimeout(ctx, tm.timeout)
	defer cancel()
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok && tx != nil {
		return fn(ctx)
	}

	tx, err := tm.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), tm.timeout)
			defer cancel()
			_ = tx.Rollback(rollbackCtx)
			panic(p)
		} else if err != nil {
			rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), tm.timeout)
			defer cancel()
			_ = tx.Rollback(rollbackCtx)
		}
	}()

	ctxWithTx := context.WithValue(ctx, txKey{}, tx)
	if err = fn(ctxWithTx); err != nil {
		return err
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit tx: %w", err)
	}
	return nil
}

func GetExecutor(ctx context.Context, pool *pgxpool.Pool) QueryExecutor {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok && tx != nil {
		return tx
	}
	return pool
}
