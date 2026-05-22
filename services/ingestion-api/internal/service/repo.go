package service

import (
	"context"
	"encoding/json"
)

type DLQRepository interface {
	Insert(ctx context.Context, raw json.RawMessage, errMsg string) error
}

type QueuePublisher interface {
	Publish(ctx context.Context, payload json.RawMessage) error
}
