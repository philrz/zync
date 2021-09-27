package cli

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"

	"gopkg.in/confluentinc/confluent-kafka-go.v1/kafka"
)

type Flags struct {
	Topic       string
	Namespace   string
	ZedLakeHost string
}

type Credentials struct {
	User     string
	Password string
}

const HostEnv = "ZED_LAKE_HOST"

func DefaultHost() string {
	host := os.Getenv(HostEnv)
	if host == "" {
		host = "localhost:9867"
	}
	return host
}

func (f *Flags) SetFlags(fs *flag.FlagSet) {
	fs.StringVar(&f.Topic, "topic", "", "Kafka topic name")
	fs.StringVar(&f.Namespace, "namespace", "io.brimdata.zinger", "Kafka name space for new schemas")
	fs.StringVar(&f.ZedLakeHost, "host", DefaultHost(), "host[:port] of Zed lake service")
}

func SchemaRegistryEndpoint() (string, Credentials, error) {
	key, err := getKey()
	if err != nil {
		return "", Credentials{}, err
	}
	return key.URL, Credentials{key.User, key.Password}, nil
}

type apiKey struct {
	URL      string `json:"url"`
	User     string `json:"user"`
	Password string `json:"password"`
}

func getKey() (apiKey, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return apiKey{}, err
	}
	path := filepath.Join(home, ".zinger", "schema_registry.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return apiKey{}, err
	}
	var key apiKey
	err = json.Unmarshal(b, &key)
	return key, err
}

type config struct {
	BootstrapServers string `json:"bootstrap_servers"`
	SecurityProtocol string `json:"security_protocol"`
	SaslMechanisms   string `json:"sasl_mechanisms"`
	SaslUsername     string `json:"sasl_username"`
	SaslPassword     string `json:"sasl_password"`
}

func LoadKafkaConfig() (*kafka.ConfigMap, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(home, ".zinger", "kafka.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c config
	err = json.Unmarshal(b, &c)
	return &kafka.ConfigMap{
		"bootstrap.servers": c.BootstrapServers,
		"sasl.mechanisms":   c.SaslMechanisms,
		"security.protocol": c.SecurityProtocol,
		"sasl.username":     c.SaslUsername,
		"sasl.password":     c.SaslPassword,
	}, err
}
