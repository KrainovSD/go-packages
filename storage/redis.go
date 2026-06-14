package storage

import (
	"context"
	"fmt"
	"strings"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
)

type CreateRedisOptions struct {
	Mode       string // standalone, sentinel, cluster
	Addresses  []string
	MasterName string
	Username   string
	Password   string
	DB         int
	Tracing    bool
	Metrics    bool
}

func CreateRedis(ctx context.Context, opts *CreateRedisOptions) (redis.UniversalClient, error) {
	if len(opts.Addresses) == 0 {
		return nil, fmt.Errorf("empty redis addresses")
	}

	var client redis.UniversalClient
	switch strings.ToLower(opts.Mode) {
	case "sentinel":
		client = redis.NewFailoverClient(&redis.FailoverOptions{
			SentinelAddrs: opts.Addresses,
			MasterName:    opts.MasterName,
			Username:      opts.Username,
			Password:      opts.Password,
			DB:            opts.DB,
		})
	case "cluster":
		client = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:    opts.Addresses,
			Username: opts.Username,
			Password: opts.Password,
		})
	default:
		client = redis.NewClient(&redis.Options{
			Addr:     opts.Addresses[0],
			Username: opts.Username,
			Password: opts.Password,
			DB:       opts.DB,
		})
	}

	var err error
	if opts.Tracing {
		if err = redisotel.InstrumentTracing(client); err != nil {
			return nil, fmt.Errorf("register tracing: %w", err)
		}
	}
	if opts.Metrics {
		if err = redisotel.InstrumentMetrics(client); err != nil {
			return nil, fmt.Errorf("register metrics: %w", err)
		}
	}
	if err = client.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	return client, nil
}
