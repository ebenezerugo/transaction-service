package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

type DLQMessage struct {
	OriginalTopic     string          `json:"original_topic"`
	OriginalPartition int32           `json:"original_partition"`
	OriginalOffset    int64           `json:"original_offset"`
	OriginalKey       string          `json:"original_key"`
	OriginalValue     json.RawMessage `json:"original_value"`
	Error             string          `json:"error"`
	FailedAt          time.Time       `json:"failed_at"`
	RetryCount        int             `json:"retry_count"`
}

type DLQProducer struct {
	producer *kafka.Producer
	topic    string
}

func NewDLQProducer(brokers string) (*DLQProducer, error) {
	p, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers": brokers,
	})
	if err != nil {
		return nil, fmt.Errorf("create DLQ producer: %w", err)
	}
	return &DLQProducer{producer: p, topic: TopicDLQ}, nil
}

func (d *DLQProducer) PublishToDLQ(ctx context.Context, msg *kafka.Message, err error) error {
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	dlqMsg := DLQMessage{
		OriginalKey:   string(msg.Key),
		OriginalValue: json.RawMessage(msg.Value),
		Error:         errMsg,
		FailedAt:      time.Now(),
	}
	if msg.TopicPartition.Topic != nil {
		dlqMsg.OriginalTopic = *msg.TopicPartition.Topic
	}
	dlqMsg.OriginalPartition = msg.TopicPartition.Partition
	dlqMsg.OriginalOffset = int64(msg.TopicPartition.Offset)

	data, _ := json.Marshal(dlqMsg)
	topic := d.topic
	deliveryChan := make(chan kafka.Event, 1)
	_ = d.producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Key:            msg.Key,
		Value:          data,
		Timestamp:      time.Now(),
	}, deliveryChan)

	select {
	case e := <-deliveryChan:
		m := e.(*kafka.Message)
		if m.TopicPartition.Error != nil {
			return fmt.Errorf("DLQ delivery failed: %w", m.TopicPartition.Error)
		}
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

func (d *DLQProducer) Close() {
	d.producer.Flush(5000)
	d.producer.Close()
}
