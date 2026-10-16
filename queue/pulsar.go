package queue

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/apache/pulsar-client-go/pulsar"
	pulsarlog "github.com/apache/pulsar-client-go/pulsar/log"
)

type PulsarSecurityOptions struct {
	Token               string
	TokenFile           string
	User                string
	Password            string
	TLSCaLocation       string
	TLSLocation         string
	TLSKeyLocation      string
	TLSAllowInsecure    bool
	TLSValidateHostname bool
}

type PulsarClientOptions struct {
	URL               string
	ListenerName      string
	SecurityOptions   PulsarSecurityOptions
	ConnectionTimeout time.Duration
	OperationTimeout  time.Duration
	Logger            *slog.Logger
}

func NewPulsarClient(opts *PulsarClientOptions) (pulsar.Client, error) {
	if opts.URL == "" {
		return nil, fmt.Errorf("pulsar client: URL required")
	}
	var clientOptions = pulsar.ClientOptions{
		URL:                        opts.URL,
		TLSAllowInsecureConnection: opts.SecurityOptions.TLSAllowInsecure,
		TLSValidateHostname:        opts.SecurityOptions.TLSValidateHostname,
	}
	if opts.ListenerName != "" {
		clientOptions.ListenerName = opts.ListenerName
	}
	if opts.SecurityOptions.TLSCaLocation != "" {
		clientOptions.TLSTrustCertsFilePath = opts.SecurityOptions.TLSCaLocation
	}
	if opts.ConnectionTimeout > 0 {
		clientOptions.ConnectionTimeout = opts.ConnectionTimeout
	}
	if opts.OperationTimeout > 0 {
		clientOptions.OperationTimeout = opts.OperationTimeout
	}
	if opts.Logger != nil {
		clientOptions.Logger = pulsarlog.NewLoggerWithSlog(opts.Logger)
	}
	var err error
	if clientOptions.Authentication, err = newPulsarAuthentication(&opts.SecurityOptions, opts.Logger); err != nil {
		return nil, err
	}
	var client pulsar.Client
	if client, err = pulsar.NewClient(clientOptions); err != nil {
		return nil, fmt.Errorf("create pulsar client: %w", err)
	}
	return client, nil
}

type PulsarProducerOptions struct {
	Topic                   string
	Name                    string
	SendTimeout             time.Duration
	MaxPendingMessages      int
	DisableBlockIfQueueFull bool
	Compression             string
}

func NewPulsarProducer(client pulsar.Client, opts *PulsarProducerOptions) (pulsar.Producer, error) {
	var producerOptions = pulsar.ProducerOptions{Topic: opts.Topic}
	if opts.Name != "" {
		producerOptions.Name = opts.Name
	}
	if opts.SendTimeout > 0 {
		producerOptions.SendTimeout = opts.SendTimeout
	}
	if opts.MaxPendingMessages > 0 {
		producerOptions.MaxPendingMessages = opts.MaxPendingMessages
	}
	producerOptions.DisableBlockIfQueueFull = opts.DisableBlockIfQueueFull
	var compressionName = opts.Compression
	if compressionName == "" {
		compressionName = "zstd"
	}
	var err error
	var compression pulsar.CompressionType
	if compression, err = newPulsarCompression(compressionName); err != nil {
		return nil, err
	}
	producerOptions.CompressionType = compression
	var producer pulsar.Producer
	if producer, err = client.CreateProducer(producerOptions); err != nil {
		return nil, fmt.Errorf("create pulsar producer for topic %s: %w", opts.Topic, err)
	}
	return producer, nil
}

type PulsarConsumerOptions struct {
	Topics           []string
	SubscriptionName string
	Name             string
	SubscriptionType string
	InitialPosition  string
}

func NewPulsarConsumer(client pulsar.Client, opts *PulsarConsumerOptions) (pulsar.Consumer, error) {
	var consumerOptions = pulsar.ConsumerOptions{
		Topics:           opts.Topics,
		SubscriptionName: opts.SubscriptionName,
	}
	if opts.Name != "" {
		consumerOptions.Name = opts.Name
	}
	var err error
	if opts.SubscriptionType != "" {
		var subscriptionType pulsar.SubscriptionType
		if subscriptionType, err = newPulsarSubscriptionType(opts.SubscriptionType); err != nil {
			return nil, err
		}
		consumerOptions.Type = subscriptionType
	}
	if opts.InitialPosition != "" {
		var position pulsar.SubscriptionInitialPosition
		if position, err = newPulsarInitialPosition(opts.InitialPosition); err != nil {
			return nil, err
		}
		consumerOptions.SubscriptionInitialPosition = position
	}
	var consumer pulsar.Consumer
	if consumer, err = client.Subscribe(consumerOptions); err != nil {
		return nil, fmt.Errorf("subscribe pulsar topics %v: %w", opts.Topics, err)
	}
	return consumer, nil
}

func newPulsarAuthentication(s *PulsarSecurityOptions, logger *slog.Logger) (pulsar.Authentication, error) {
	var kinds []string
	if s.Token != "" {
		kinds = append(kinds, "token")
	}
	if s.TokenFile != "" {
		kinds = append(kinds, "token_file")
	}
	if s.User != "" || s.Password != "" {
		if s.User == "" || s.Password == "" {
			return nil, fmt.Errorf("pulsar basic auth requires both User and Password")
		}
		kinds = append(kinds, "basic")
	}
	if s.TLSLocation != "" || s.TLSKeyLocation != "" {
		if s.TLSLocation == "" || s.TLSKeyLocation == "" {
			return nil, fmt.Errorf("pulsar tls auth requires both TLSLocation and TLSKeyLocation")
		}
		kinds = append(kinds, "tls")
	}
	if len(kinds) == 0 {
		return nil, nil
	}
	if len(kinds) > 1 && logger != nil {
		logger.Warn("multiple pulsar auth mechanisms configured, using the highest priority",
			"chosen", kinds[0], "ignored", strings.Join(kinds[1:], ", "))
	}
	switch kinds[0] {
	case "token":
		return pulsar.NewAuthenticationToken(s.Token), nil
	case "token_file":
		return pulsar.NewAuthenticationTokenFromFile(s.TokenFile), nil
	case "basic":
		var authentication pulsar.Authentication
		var err error
		if authentication, err = pulsar.NewAuthenticationBasic(s.User, s.Password); err != nil {
			return nil, fmt.Errorf("create pulsar basic auth: %w", err)
		}
		return authentication, nil
	default:
		return pulsar.NewAuthenticationTLS(s.TLSLocation, s.TLSKeyLocation), nil
	}
}

func newPulsarCompression(name string) (pulsar.CompressionType, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "zstd":
		return pulsar.ZSTD, nil
	case "lz4":
		return pulsar.LZ4, nil
	case "zlib":
		return pulsar.ZLib, nil
	case "snappy":
		return pulsar.SNAPPY, nil
	case "none":
		return pulsar.NoCompression, nil
	default:
		return pulsar.NoCompression, fmt.Errorf("unknown pulsar compression %q: expected zstd, lz4, zlib, snappy or none", name)
	}
}

func newPulsarSubscriptionType(name string) (pulsar.SubscriptionType, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "shared":
		return pulsar.Shared, nil
	case "failover":
		return pulsar.Failover, nil
	case "exclusive":
		return pulsar.Exclusive, nil
	case "key_shared":
		return pulsar.KeyShared, nil
	default:
		return pulsar.Exclusive, fmt.Errorf("unknown pulsar subscription type %q: expected shared, failover, exclusive or key_shared", name)
	}
}

func newPulsarInitialPosition(name string) (pulsar.SubscriptionInitialPosition, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "latest":
		return pulsar.SubscriptionPositionLatest, nil
	case "earliest":
		return pulsar.SubscriptionPositionEarliest, nil
	default:
		return pulsar.SubscriptionPositionLatest, fmt.Errorf("unknown pulsar initial position %q: expected latest or earliest", name)
	}
}
