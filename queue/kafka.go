package queue

import (
	"context"
	"fmt"
	"strings"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

type KafkaSecurityOptions struct {
	SecurityProtocol string // SSL, SASL_SSL, SASL_PLAINTEXT, PLAINTEXT
	TLSCaLocation    string
	TLSLocation      string
	TLSKeyLocation   string
	SASLMechanism    string // GSSAPI, PLAIN
	SASLKeytabPath   string
	SASLPrincipal    string
	User             string
	Password         string
}

type KafkaProducerOptions struct {
	SecurityOptions    KafkaSecurityOptions
	Servers            []string
	Linger             int
	Compression        string
	DisableIdempotence bool
	MessageTimeoutMs   int
	RequestTimeoutMs   int
	ClientID           string
}

func NewKafkaProducer(ctx context.Context, opts *KafkaProducerOptions) (*kafka.Producer, error) {
	var linger = 5
	if opts.Linger != 0 {
		linger = opts.Linger
	}
	var cm = &kafka.ConfigMap{
		"bootstrap.servers":  strings.Join(opts.Servers, ","),
		"linger.ms":          linger,
		"enable.idempotence": true,
		"compression.type":   "zstd",
	}
	if opts.Compression != "" {
		cm.SetKey("compression.type", strings.ToLower(opts.Compression))
	}
	if opts.MessageTimeoutMs != 0 {
		cm.SetKey("message.timeout.ms", opts.MessageTimeoutMs)
	}
	if opts.RequestTimeoutMs != 0 {
		cm.SetKey("request.timeout.ms", opts.RequestTimeoutMs)
	}
	if opts.DisableIdempotence {
		cm.SetKey("acks", "1")
		cm.SetKey("enable.idempotence", false)
	}
	if opts.ClientID != "" {
		cm.SetKey("client.id", opts.ClientID)
	}
	applySecurity(cm, &opts.SecurityOptions)
	var producer *kafka.Producer
	var err error
	if producer, err = kafka.NewProducer(cm); err != nil {
		return producer, fmt.Errorf("create kafka producer: %w", err)
	}
	return producer, nil
}

type KafkaConsumerOptions struct {
	SecurityOptions KafkaSecurityOptions
	Servers         []string
	ConsumerGroup   string
	ConsumeTopics   []string
	AutoOffsetReset string // earliest, latest
	ClientID        string
}

func NewKafkaConsumer(ctx context.Context, opts *KafkaConsumerOptions) (*kafka.Consumer, error) {
	var cm = &kafka.ConfigMap{
		"bootstrap.servers":  strings.Join(opts.Servers, ","),
		"group.id":           opts.ConsumerGroup,
		"enable.auto.commit": false,
	}
	if opts.AutoOffsetReset != "" {
		cm.SetKey("auto.offset.reset", strings.ToLower(opts.AutoOffsetReset))
	}
	if opts.ClientID != "" {
		cm.SetKey("client.id", opts.ClientID)
	}
	applySecurity(cm, &opts.SecurityOptions)
	var consumer *kafka.Consumer
	var err error
	if consumer, err = kafka.NewConsumer(cm); err != nil {
		return consumer, fmt.Errorf("create kafka consumer: %w", err)
	}
	if err = consumer.SubscribeTopics(opts.ConsumeTopics, nil); err != nil {
		_ = consumer.Close()
		return consumer, fmt.Errorf("subscribe topics: %w", err)
	}

	return consumer, nil
}

func applySecurity(cm *kafka.ConfigMap, s *KafkaSecurityOptions) {
	if s.SecurityProtocol != "" {
		cm.SetKey("security.protocol", strings.ToUpper(s.SecurityProtocol))
	}
	if s.TLSCaLocation != "" {
		cm.SetKey("ssl.ca.location", s.TLSCaLocation)
	}
	if s.SASLMechanism != "" {
		cm.SetKey("sasl.mechanism", strings.ToUpper(s.SASLMechanism))
	}
	if s.SASLKeytabPath != "" {
		cm.SetKey("sasl.kerberos.keytab", s.SASLKeytabPath)
	}
	if s.SASLPrincipal != "" {
		cm.SetKey("sasl.kerberos.principal", s.SASLPrincipal)
	}
	if s.TLSLocation != "" {
		cm.SetKey("ssl.certificate.location", s.TLSLocation)
	}
	if s.TLSKeyLocation != "" {
		cm.SetKey("ssl.key.location", s.TLSKeyLocation)
	}
	if s.User != "" {
		cm.SetKey("sasl.username", s.User)
	}
	if s.Password != "" {
		cm.SetKey("sasl.password", s.Password)
	}
}
