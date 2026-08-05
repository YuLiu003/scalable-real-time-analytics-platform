package kafkaclient

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/IBM/sarama"
)

func TestFromEnvironmentValidatesTLSInputs(t *testing.T) {
	for _, name := range []string{"KAFKA_BROKERS", "KAFKA_TLS_CA_FILE", "KAFKA_TLS_CERT_FILE", "KAFKA_TLS_KEY_FILE"} {
		t.Setenv(name, "")
	}
	if _, err := FromEnvironment(); err == nil || !strings.Contains(err.Error(), "KAFKA_BROKERS") {
		t.Fatalf("FromEnvironment() missing brokers error = %v", err)
	}
	t.Setenv("KAFKA_BROKERS", " broker-a:9093, ,broker-b:9093 ")
	if _, err := FromEnvironment(); err == nil || !strings.Contains(err.Error(), "KAFKA_TLS_CA_FILE") {
		t.Fatalf("FromEnvironment() missing TLS error = %v", err)
	}
	t.Setenv("KAFKA_TLS_CA_FILE", "/ca")
	t.Setenv("KAFKA_TLS_CERT_FILE", "/cert")
	t.Setenv("KAFKA_TLS_KEY_FILE", "/key")
	config, err := FromEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(config.Brokers, []string{"broker-a:9093", "broker-b:9093"}) ||
		config.CAFile != "/ca" || config.CertFile != "/cert" || config.KeyFile != "/key" {
		t.Fatalf("config = %#v", config)
	}
}

func TestProducerAndConsumerUseReviewedContracts(t *testing.T) {
	config := validTLSConfig(t)
	producer, err := config.Producer("private-producer")
	if err != nil {
		t.Fatal(err)
	}
	if producer.ClientID != "private-producer" || producer.Version != sarama.V3_8_0_0 ||
		producer.Net.MaxOpenRequests != 1 || !producer.Producer.Idempotent ||
		producer.Producer.RequiredAcks != sarama.WaitForAll || !producer.Producer.Return.Successes ||
		producer.Producer.Retry.Max != 10 || producer.Producer.Retry.Backoff != 250*time.Millisecond ||
		producer.Metadata.AllowAutoTopicCreation || !producer.Net.TLS.Enable ||
		producer.Net.TLS.Config.MinVersion == 0 || producer.Producer.Partitioner("topic") == nil {
		t.Fatalf("producer config = %#v", producer)
	}

	consumer, err := config.Consumer("private-consumer")
	if err != nil {
		t.Fatal(err)
	}
	if consumer.ClientID != "private-consumer" || consumer.Consumer.Offsets.Initial != sarama.OffsetOldest ||
		!consumer.Consumer.Return.Errors || consumer.Consumer.Group.Session.Timeout != 20*time.Second ||
		consumer.Consumer.Group.Heartbeat.Interval != 3*time.Second ||
		consumer.Consumer.Group.Rebalance.Timeout != 30*time.Second ||
		len(consumer.Consumer.Group.Rebalance.GroupStrategies) != 1 {
		t.Fatalf("consumer config = %#v", consumer)
	}
}

func TestTLSConfigurationFailuresAreExplicit(t *testing.T) {
	valid := validTLSConfig(t)
	tests := []struct {
		name   string
		mutate func(*Config)
		call   func(Config) error
		want   string
	}{
		{name: "producer CA read", mutate: func(config *Config) { config.CAFile = "/missing/ca" }, call: producerError, want: "read Kafka CA"},
		{name: "consumer empty CA", mutate: func(config *Config) {
			path := filepath.Join(t.TempDir(), "empty-ca.pem")
			if err := os.WriteFile(path, []byte("not a certificate"), 0o600); err != nil {
				t.Fatal(err)
			}
			config.CAFile = path
		}, call: consumerError, want: "contains no certificates"},
		{name: "client key pair", mutate: func(config *Config) { config.KeyFile = config.CAFile }, call: producerError, want: "load Kafka client certificate"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := valid

			test.mutate(&config)
			if err := test.call(config); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("configuration error = %v, want %q", err, test.want)
			}
		})
	}
}

func producerError(config Config) error {
	_, err := config.Producer("test")
	return err
}

func consumerError(config Config) error {
	_, err := config.Consumer("test")
	return err
}

func validTLSConfig(t *testing.T) Config {
	t.Helper()
	directory := t.TempDir()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	clientKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	clientTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "test-client"},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	clientDER, err := x509.CreateCertificate(rand.Reader, clientTemplate, caTemplate, &clientKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(clientKey)
	if err != nil {
		t.Fatal(err)
	}
	caFile := filepath.Join(directory, "ca.pem")
	certFile := filepath.Join(directory, "client.pem")
	keyFile := filepath.Join(directory, "client-key.pem")
	writePEM(t, caFile, "CERTIFICATE", caDER)
	writePEM(t, certFile, "CERTIFICATE", clientDER)
	writePEM(t, keyFile, "PRIVATE KEY", keyDER)
	return Config{Brokers: []string{"broker:9093"}, CAFile: caFile, CertFile: certFile, KeyFile: keyFile}
}

func writePEM(t *testing.T, path, blockType string, data []byte) {
	t.Helper()
	encoded := pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: data})
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
}
