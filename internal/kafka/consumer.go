package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/ebenezerugo/transaction-service/internal/domain"
)

type MessageHandler func(ctx context.Context, msg *kafka.Message) error

type Consumer interface {
	Subscribe(topics []string) error
	Poll(timeout int) (kafka.Event, error)
	Commit() error
	Close() error
}

type kafkaConsumer struct {
	consumer *kafka.Consumer
}

func NewConsumer(brokers, groupID string) (Consumer, error) {
	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":  brokers,
		"group.id":           groupID,
		"auto.offset.reset":  "earliest",
		"enable.auto.commit": false,
	})
	if err != nil {
		return nil, fmt.Errorf("create kafka consumer: %w", err)
	}
	return &kafkaConsumer{consumer: c}, nil
}

func (c *kafkaConsumer) Subscribe(topics []string) error {
	return c.consumer.SubscribeTopics(topics, nil)
}

func (c *kafkaConsumer) Poll(timeout int) (kafka.Event, error) {
	ev := c.consumer.Poll(timeout)
	return ev, nil
}

func (c *kafkaConsumer) Commit() error {
	_, err := c.consumer.Commit()
	return err
}

func (c *kafkaConsumer) Close() error {
	return c.consumer.Close()
}

type RetryConsumer struct {
	consumer         Consumer
	handler          MessageHandler
	dlqProducer      *DLQProducer
	maxAttempts      int
	initialBackoffMs int
	maxBackoffMs     int
}

func NewRetryConsumer(consumer Consumer, handler MessageHandler, dlqProducer *DLQProducer, maxAttempts, initialBackoffMs, maxBackoffMs int) *RetryConsumer {
	return &RetryConsumer{
		consumer:         consumer,
		handler:          handler,
		dlqProducer:      dlqProducer,
		maxAttempts:      maxAttempts,
		initialBackoffMs: initialBackoffMs,
		maxBackoffMs:     maxBackoffMs,
	}
}

func (c *RetryConsumer) processWithRetry(ctx context.Context, msg *kafka.Message) error {
	var lastErr error
	for attempt := 0; attempt < c.maxAttempts; attempt++ {
		if attempt > 0 {
			backoff := calculateBackoff(attempt, c.initialBackoffMs, c.maxBackoffMs)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}
		if err := c.handler(ctx, msg); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	_ = c.dlqProducer.PublishToDLQ(ctx, msg, lastErr)
	return domain.ErrMaxRetriesExceeded
}

func calculateBackoff(attempt, initial, max int) time.Duration {
	backoff := initial * (1 << uint(attempt))
	if backoff > max {
		backoff = max
	}
	return time.Duration(backoff) * time.Millisecond
}

func (c *RetryConsumer) Start(ctx context.Context, topics []string) error {
	if err := c.consumer.Subscribe(topics); err != nil {
		return fmt.Errorf("subscribe topics: %w", err)
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		ev, _ := c.consumer.Poll(100)
		if ev == nil {
			continue
		}
		switch e := ev.(type) {
		case *kafka.Message:
			_ = c.processWithRetry(ctx, e)
			_ = c.consumer.Commit()
		case kafka.Error:
			if e.IsFatal() {
				return fmt.Errorf("fatal kafka error: %w", e)
			}
		}
	}
}
