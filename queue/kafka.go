package queue

import (
	"context"
	"fmt"
	"strings"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

type SecurityOptions struct {
	SecurityProtocol string // SSL, SASL_SSL, SASL_PLAINTEXT, PLAINTEXT
	SslCaLocation    string
	SslLocation      string
	SslKeyLocation   string
	Mechanism        string // GSSAPI, PLAIN
	KeytabPath       string
	Principal        string
	User             string
	Password         string
}

type CreateProducerOptions struct {
	SecurityOptions SecurityOptions
	Servers         []string
	Linger          int
}

func CreateProducer(ctx context.Context, opts *CreateProducerOptions) (*kafka.Producer, error) {
	var linger = 5
	if opts.Linger != 0 {
		linger = opts.Linger
	}
	var cm = &kafka.ConfigMap{
		"bootstrap.servers":  strings.Join(opts.Servers, ","),
		"linger.ms":          linger,
		"compression.type":   "zstd",
		"request.timeout.ms": 15000,
		"message.timeout.ms": 60000,
		"enable.idempotence": true,
		"acks":               "all",
	}
	applySecurity(cm, &opts.SecurityOptions)
	var producer *kafka.Producer
	var err error
	if producer, err = kafka.NewProducer(cm); err != nil {
		return nil, fmt.Errorf("create kafka producer: %w", err)
	}
	return producer, nil
}

type CreateConsumerOptions struct {
	SecurityOptions SecurityOptions
	Servers         []string
	ConsumerGroup   string
	ConsumeTopics   []string
	AutoOffsetReset string // earliest, latest
}

func CreateConsumer(ctx context.Context, opts *CreateConsumerOptions) (*kafka.Consumer, error) {
	var cm = &kafka.ConfigMap{
		"bootstrap.servers":  strings.Join(opts.Servers, ","),
		"group.id":           opts.ConsumerGroup,
		"enable.auto.commit": false,
	}
	if opts.AutoOffsetReset != "" {
		cm.SetKey("auto.offset.reset", strings.ToLower(opts.AutoOffsetReset))
	}
	applySecurity(cm, &opts.SecurityOptions)
	var consumer *kafka.Consumer
	var err error
	if consumer, err = kafka.NewConsumer(cm); err != nil {
		return nil, fmt.Errorf("create kafka consumer: %w", err)
	}
	if err = consumer.SubscribeTopics(opts.ConsumeTopics, nil); err != nil {
		_ = consumer.Close()
		return nil, fmt.Errorf("subscribe topics: %w", err)
	}

	return consumer, nil
}

func applySecurity(cm *kafka.ConfigMap, s *SecurityOptions) {
	if s.SecurityProtocol != "" {
		cm.SetKey("security.protocol", strings.ToUpper(s.SecurityProtocol))
	}
	if s.SslCaLocation != "" {
		cm.SetKey("ssl.ca.location", s.SslCaLocation)
	}
	if s.Mechanism != "" {
		cm.SetKey("sasl.mechanism", strings.ToUpper(s.Mechanism))
	}
	if s.KeytabPath != "" {
		cm.SetKey("sasl.kerberos.keytab", s.KeytabPath)
	}
	if s.Principal != "" {
		cm.SetKey("sasl.kerberos.principal", s.Principal)
	}
	if s.SslLocation != "" {
		cm.SetKey("ssl.certificate.location", s.SslLocation)
	}
	if s.SslKeyLocation != "" {
		cm.SetKey("ssl.key.location", s.SslKeyLocation)
	}
	if s.User != "" {
		cm.SetKey("sasl.username", s.User)
	}
	if s.Password != "" {
		cm.SetKey("sasl.password", s.Password)
	}
}
