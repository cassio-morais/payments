package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	PaymentStream = "payments:processing"
	WebhookStream = "webhooks:delivery"
	DLQStream     = "payments:dlq"
)

type StreamProducer struct {
	client *redis.Client
}

func NewStreamProducer(client *redis.Client) *StreamProducer {
	return &StreamProducer{client: client}
}

func (p *StreamProducer) PublishPaymentEvent(ctx context.Context, paymentID string, eventType string, data map[string]any) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	args := &redis.XAddArgs{
		Stream: PaymentStream,
		Values: map[string]any{
			"payment_id": paymentID,
			"event_type": eventType,
			"payload":    string(payload),
			"timestamp":  time.Now().Unix(),
		},
	}

	_, err = p.client.XAdd(ctx, args).Result()
	if err != nil {
		return fmt.Errorf("failed to publish payment event: %w", err)
	}

	return nil
}

func (p *StreamProducer) PublishWebhookEvent(ctx context.Context, webhookID string, data map[string]any) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook data: %w", err)
	}

	args := &redis.XAddArgs{
		Stream: WebhookStream,
		Values: map[string]any{
			"webhook_id": webhookID,
			"payload":    string(payload),
			"timestamp":  time.Now().Unix(),
		},
	}

	_, err = p.client.XAdd(ctx, args).Result()
	if err != nil {
		return fmt.Errorf("failed to publish webhook event: %w", err)
	}

	return nil
}

func (p *StreamProducer) PublishToDLQ(ctx context.Context, paymentID string, reason string, originalData map[string]any) error {
	payload, err := json.Marshal(originalData)
	if err != nil {
		return fmt.Errorf("failed to marshal DLQ data: %w", err)
	}

	args := &redis.XAddArgs{
		Stream: DLQStream,
		Values: map[string]any{
			"payment_id": paymentID,
			"reason":     reason,
			"payload":    string(payload),
			"timestamp":  time.Now().Unix(),
		},
	}

	_, err = p.client.XAdd(ctx, args).Result()
	if err != nil {
		return fmt.Errorf("failed to publish to DLQ: %w", err)
	}

	return nil
}

type StreamConsumer struct {
	client        *redis.Client
	stream        string
	group         string
	consumer      string
	batchSize     int64
	blockDuration time.Duration
}

func NewStreamConsumer(
	client *redis.Client,
	stream string,
	group string,
	consumer string,
	batchSize int64,
	blockDuration time.Duration,
) *StreamConsumer {
	return &StreamConsumer{
		client:        client,
		stream:        stream,
		group:         group,
		consumer:      consumer,
		batchSize:     batchSize,
		blockDuration: blockDuration,
	}
}

func (c *StreamConsumer) CreateGroup(ctx context.Context) error {
	// Create stream if it doesn't exist
	const busyGroupMsg = "BUSYGROUP"
	err := c.client.XGroupCreateMkStream(ctx, c.stream, c.group, "0").Err()
	if err != nil && !strings.Contains(err.Error(), busyGroupMsg) {
		return fmt.Errorf("failed to create consumer group: %w", err)
	}
	return nil
}

func (c *StreamConsumer) Read(ctx context.Context) ([]redis.XStream, error) {
	streams, err := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    c.group,
		Consumer: c.consumer,
		Streams:  []string{c.stream, ">"},
		Count:    c.batchSize,
		Block:    c.blockDuration,
	}).Result()

	if err != nil {
		if err == redis.Nil {
			// No new messages
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read from stream: %w", err)
	}

	return streams, nil
}

func (c *StreamConsumer) Ack(ctx context.Context, messageID string) error {
	err := c.client.XAck(ctx, c.stream, c.group, messageID).Err()
	if err != nil {
		return fmt.Errorf("failed to ack message: %w", err)
	}
	return nil
}

func (c *StreamConsumer) Claim(ctx context.Context, minIdleTime time.Duration, messageIDs []string) ([]redis.XMessage, error) {
	messages, err := c.client.XClaim(ctx, &redis.XClaimArgs{
		Stream:   c.stream,
		Group:    c.group,
		Consumer: c.consumer,
		MinIdle:  minIdleTime,
		Messages: messageIDs,
	}).Result()

	if err != nil {
		return nil, fmt.Errorf("failed to claim messages: %w", err)
	}

	return messages, nil
}
