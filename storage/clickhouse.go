package storage

import (
	"context"
	"fmt"
	"net/http"

	"github.com/ClickHouse/clickhouse-go/v2"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type ClickhouseOptions struct {
	// max_open_conns, max_idle_conns, dial_timeout, conn_max_lifetime
	Connection string
	// only for http
	Tracing bool
}

func NewClickhouse(ctx context.Context, opts *ClickhouseOptions) (clickhouse.Conn, error) {
	var clickOpts, err = clickhouse.ParseDSN(opts.Connection)
	if err != nil {
		return nil, fmt.Errorf("parse clickhouse dsn: %w", err)
	}
	if opts.Tracing {
		clickOpts.TransportFunc = func(t *http.Transport) (http.RoundTripper, error) {
			return otelhttp.NewTransport(t), nil
		}
	}
	var conn clickhouse.Conn
	if conn, err = clickhouse.Open(clickOpts); err != nil {
		return nil, fmt.Errorf("open clickhouse: %w", err)
	}
	if err = ping(ctx, conn); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}
	return conn, nil
}

func ping(ctx context.Context, conn clickhouse.Conn) error {
	if err := conn.Ping(ctx); err != nil {
		return fmt.Errorf("clickhouse ping: %w", err)
	}
	return nil
}
