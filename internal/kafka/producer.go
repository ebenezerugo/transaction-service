package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

type Producer interface {
	PublishTransaction(ctx context.Context, topic string, accountID string, msg interface{}) error
	Close() error
}

type kafkaProducer struct {
	producer *kafka.Producer
}

func NewProducer(brokers string) (Producer, error) {
	p, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers": brokers,
		"acks":              "all",
		"retries":           3,
	})
	if err != nil {
		return nil, fmt.Errorf("create kafka producer: %w", err)
	}
	return &kafkaProducer{producer: p}, nil
}

func (p *kafkaProducer) PublishTransaction(ctx context.Context, topic string, accountID string, msg interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	deliveryChan := make(chan kafka.Event, 1)
	err = p.producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Key:            []byte(accountID),
		Value:          data,
		Timestamp:      time.Now(),
	}, deliveryChan)
	if err != nil {
		return fmt.Errorf("produce message: %w", err)
	}

	select {
	case e := <-deliveryChan:
		m := e.(*kafka.Message)
		if m.TopicPartition.Error != nil {
			return fmt.Errorf("delivery failed: %w", m.TopicPartition.Error)
		}
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

func (p *kafkaProducer) Close() error {
	p.producer.Flush(5000)
	p.producer.Close()
	return nil
}
