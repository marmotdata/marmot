package kafka

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/schemaregistry"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
)

func (s *Source) initClient(ctx context.Context) error {
	opts := []kgo.Opt{
		kgo.SeedBrokers(strings.Split(s.config.BootstrapServers, ",")...),
	}

	if s.config.ClientID != "" {
		opts = append(opts, kgo.ClientID(s.config.ClientID))
	}

	if s.config.ClientTimeout > 0 {
		timeout := time.Duration(s.config.ClientTimeout) * time.Second
		opts = append(opts, kgo.RequestTimeoutOverhead(timeout))
	}

	if s.config.Authentication != nil {
		authOpts, err := s.configureAuthentication()
		if err != nil {
			return fmt.Errorf("configuring authentication: %w", err)
		}
		opts = append(opts, authOpts...)
	}

	client, err := kgo.NewClient(opts...)
	if err != nil {
		return fmt.Errorf("creating Kafka client: %w", err)
	}
	s.client = client

	s.admin = kadm.NewClient(client)

	testCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	_, err = s.admin.ListTopics(testCtx)
	if err != nil {
		if strings.Contains(err.Error(), "EOF") {
			return fmt.Errorf("connection closed unexpectedly (EOF): this usually indicates an authentication failure, incorrect credentials, or network connectivity issues")
		}
		if strings.Contains(err.Error(), "timed out") {
			return fmt.Errorf("connection timed out: check your network connectivity and firewall settings")
		}
		if strings.Contains(err.Error(), "authentication") {
			return fmt.Errorf("authentication failed: check your username, password, and SASL mechanism")
		}
		return err
	}

	return nil
}

func (s *Source) initSchemaRegistry() error {
	if s.config.SchemaRegistry.URL == "" {
		return fmt.Errorf("schema registry URL is required")
	}

	conf := schemaregistry.NewConfig(s.config.SchemaRegistry.URL)

	if userInfo, ok := s.config.SchemaRegistry.Config["basic.auth.user.info"]; ok {
		conf.BasicAuthUserInfo = userInfo
	}

	if timeout, ok := s.config.SchemaRegistry.Config["request.timeout.ms"]; ok {
		if val, err := strconv.Atoi(timeout); err == nil {
			conf.RequestTimeoutMs = val
		}
	}

	if cacheCapacity, ok := s.config.SchemaRegistry.Config["cache.capacity"]; ok {
		if val, err := strconv.Atoi(cacheCapacity); err == nil {
			conf.CacheCapacity = val
		}
	}

	client, err := schemaregistry.NewClient(conf)
	if err != nil {
		return fmt.Errorf("creating Schema Registry client: %w", err)
	}

	s.schemaRegistry = client
	return nil
}

func (s *Source) closeClient() {
	if s.client != nil {
		s.client.Close()
	}
}
