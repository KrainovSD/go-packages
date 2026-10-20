//go:build !cgo

package queue

import (
	"context"
	"errors"
)

func NewKafkaProducer(ctx context.Context, opts *KafkaProducerOptions) error {
	return errors.New("queue: kafka producer is not available in nocgo builds — build with CGO_ENABLED=1 to use kafka")
}

func NewKafkaConsumer(ctx context.Context, opts *KafkaConsumerOptions) error {
	return errors.New("queue: kafka consumer is not available in nocgo builds — build with CGO_ENABLED=1 to use kafka")
}
