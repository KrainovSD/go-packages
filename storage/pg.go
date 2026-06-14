package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

type CreatePgOptions struct {
	Connection string
	Tracing    bool
}

func CreatePg(ctx context.Context, opts *CreatePgOptions) (*pgxpool.Pool, error) {
	var pool *pgxpool.Pool
	var err error
	var config *pgxpool.Config
	if config, err = pgxpool.ParseConfig(opts.Connection); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if opts.Tracing {
		config.ConnConfig.Tracer = &OtelPgxTracer{}
	}
	if pool, err = pgxpool.NewWithConfig(ctx, config); err != nil {
		return nil, fmt.Errorf("create pg pool: %w", err)
	}
	if err = pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("pg ping check: %w", err)
	}
	if err = warmPool(ctx, pool); err != nil {
		return nil, fmt.Errorf("warm connections: %w", err)
	}
	return pool, nil
}

func warmPool(ctx context.Context, pool *pgxpool.Pool) error {
	var connections = make([]*pgxpool.Conn, 0, pool.Config().MinConns)
	var err error
	for i := range pool.Config().MinConns {
		var conn *pgxpool.Conn
		if conn, err = pool.Acquire(ctx); err != nil {
			return fmt.Errorf("get connection %d: %w", i, err)
		}
		if _, err = conn.Exec(ctx, "select 1"); err != nil {
			return fmt.Errorf("exec connection %d: %w", i, err)
		}
		connections = append(connections, conn)
	}
	for _, conn := range connections {
		conn.Release()
	}
	return nil
}

type OtelPgxTracer struct{}

func (t *OtelPgxTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	var traceCtx, span = otel.Tracer("pgx").Start(ctx, "pg")
	return context.WithValue(traceCtx, "span", span)
}

func (t *OtelPgxTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	var span trace.Span
	var ok bool
	if span, ok = ctx.Value("span").(trace.Span); ok {
		if data.Err != nil {
			span.RecordError(data.Err)
		}
		span.End()
	}
}
