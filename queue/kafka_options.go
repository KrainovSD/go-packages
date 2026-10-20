package queue

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

type KafkaConsumerOptions struct {
	SecurityOptions KafkaSecurityOptions
	Servers         []string
	ConsumerGroup   string
	ConsumeTopics   []string
	AutoOffsetReset string // earliest, latest
	ClientID        string
}
