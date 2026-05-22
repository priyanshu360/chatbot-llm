package repo

import (
	"context"
	"encoding/json"

	"github.com/go-redis/redis/v8"
)

const StreamKey = "inference:logs"
const MaxLen = 50000
const ConsumerGroup = "inference-workers"

type QueueProducer struct {
	client *redis.Client
}

func NewQueueProducer(client *redis.Client) *QueueProducer {
	return &QueueProducer{client: client}
}

func (p *QueueProducer) Publish(ctx context.Context, payload json.RawMessage) error {
	return p.client.XAdd(ctx, &redis.XAddArgs{
		Stream: StreamKey,
		MaxLen: MaxLen,
		Approx: true,
		Values: map[string]interface{}{
			"payload": string(payload),
			"version": "1",
		},
	}).Err()
}

type QueueConsumer struct {
	client *redis.Client
	name   string
}

func NewQueueConsumer(client *redis.Client, name string) *QueueConsumer {
	return &QueueConsumer{client: client, name: name}
}

func (c *QueueConsumer) EnsureGroup(ctx context.Context) error {
	err := c.client.Do(ctx, "XGROUP", "CREATE", StreamKey, ConsumerGroup, "0", "MKSTREAM").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return err
	}
	return nil
}

func (c *QueueConsumer) ReadBatch(ctx context.Context, count int64) ([]redis.XMessage, error) {
	streams, err := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    ConsumerGroup,
		Consumer: c.name,
		Streams:  []string{StreamKey, ">"},
		Count:    count,
		Block:    0,
	}).Result()

	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var messages []redis.XMessage
	for _, stream := range streams {
		messages = append(messages, stream.Messages...)
	}
	return messages, nil
}

func (c *QueueConsumer) Ack(ctx context.Context, ids ...string) error {
	return c.client.XAck(ctx, StreamKey, ConsumerGroup, ids...).Err()
}
