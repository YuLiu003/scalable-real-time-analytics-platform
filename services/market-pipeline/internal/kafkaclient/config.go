package kafkaclient

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/IBM/sarama"
)

type Config struct {
	Brokers  []string
	CAFile   string
	CertFile string
	KeyFile  string
}

func FromEnvironment() (Config, error) {
	brokers := splitNonEmpty(os.Getenv("KAFKA_BROKERS"))
	if len(brokers) == 0 {
		return Config{}, errors.New("KAFKA_BROKERS is required")
	}
	config := Config{
		Brokers:  brokers,
		CAFile:   os.Getenv("KAFKA_TLS_CA_FILE"),
		CertFile: os.Getenv("KAFKA_TLS_CERT_FILE"),
		KeyFile:  os.Getenv("KAFKA_TLS_KEY_FILE"),
	}
	if config.CAFile == "" || config.CertFile == "" || config.KeyFile == "" {
		return Config{}, errors.New("KAFKA_TLS_CA_FILE, KAFKA_TLS_CERT_FILE, and KAFKA_TLS_KEY_FILE are required")
	}
	return config, nil
}

func (c Config) Producer(clientID string) (*sarama.Config, error) {
	config, err := c.base(clientID)
	if err != nil {
		return nil, err
	}
	config.Net.MaxOpenRequests = 1
	config.Producer.Idempotent = true
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Return.Successes = true
	config.Producer.Retry.Max = 10
	config.Producer.Retry.Backoff = 250 * time.Millisecond
	config.Producer.Partitioner = sarama.NewHashPartitioner
	return config, nil
}

func (c Config) Consumer(clientID string) (*sarama.Config, error) {
	config, err := c.base(clientID)
	if err != nil {
		return nil, err
	}
	config.Consumer.Offsets.Initial = sarama.OffsetOldest
	config.Consumer.Return.Errors = true
	config.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRange()}
	config.Consumer.Group.Session.Timeout = 20 * time.Second
	config.Consumer.Group.Heartbeat.Interval = 3 * time.Second
	config.Consumer.Group.Rebalance.Timeout = 30 * time.Second
	return config, nil
}

func (c Config) base(clientID string) (*sarama.Config, error) {
	tlsConfig, err := c.tlsConfig()
	if err != nil {
		return nil, err
	}
	config := sarama.NewConfig()
	// Kafka 4.x brokers remain compatible with this client protocol baseline.
	// Broker and operator versions are pinned independently in versions.lock.
	config.Version = sarama.V3_8_0_0
	config.ClientID = clientID
	config.Metadata.AllowAutoTopicCreation = false
	config.Net.TLS.Enable = true
	config.Net.TLS.Config = tlsConfig
	config.Net.DialTimeout = 10 * time.Second
	config.Net.ReadTimeout = 10 * time.Second
	config.Net.WriteTimeout = 10 * time.Second
	return config, nil
}

func (c Config) tlsConfig() (*tls.Config, error) {
	caPEM, err := os.ReadFile(c.CAFile)
	if err != nil {
		return nil, fmt.Errorf("read Kafka CA: %w", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		return nil, errors.New("Kafka CA file contains no certificates")
	}
	certificate, err := tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("load Kafka client certificate: %w", err)
	}
	return &tls.Config{
		MinVersion:   tls.VersionTLS12,
		RootCAs:      roots,
		Certificates: []tls.Certificate{certificate},
	}, nil
}

func splitNonEmpty(value string) []string {
	var values []string
	for _, item := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}
