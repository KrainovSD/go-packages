package tests

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"github.com/KrainovSD/go-packages/internal/modules/cradle"
	"github.com/confluentinc/confluent-kafka-go/kafka"
)

type TestsServiceOptions struct {
	Cradle *cradle.Cradle
}

type TestsService struct {
	cradle *cradle.Cradle
}

func CreateService(opts *TestsServiceOptions) (*TestsService, error) {
	return &TestsService{
		cradle: opts.Cradle,
	}, nil
}

func (s *TestsService) Test(ctx context.Context) error {
	var err error

	var count int
	if err = s.cradle.Db.QueryRow(ctx, "select count(*) from test").Scan(&count); err != nil {
		return fmt.Errorf("pg query: %w", err)
	}
	s.cradle.Log.Info("pg test", "table", "test", "rows", count)

	var buf = make([]byte, 32)
	rand.Read(buf)
	if err = s.cradle.Queue.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &s.cradle.Conf.KAFKA_TOPIC},
		Value:          buf,
	}, nil); err != nil {
		return fmt.Errorf("kafka produce: %w", err)
	}
	s.cradle.Log.Info("kafka test", "topic", s.cradle.Conf.KAFKA_TOPIC, "bytes", len(buf))

	var key = "test:" + fmt.Sprintf("%d", time.Now().UnixNano())
	var value = fmt.Sprintf("value-%x", buf[:4])
	if err = s.cradle.Redis.Set(ctx, key, value, 30*time.Second).Err(); err != nil {
		return fmt.Errorf("redis set: %w", err)
	}
	var got string
	if got, err = s.cradle.Redis.Get(ctx, key).Result(); err != nil {
		return fmt.Errorf("redis get: %w", err)
	}
	s.cradle.Log.Info("redis test", "key", key, "value", got)

	return nil
}
