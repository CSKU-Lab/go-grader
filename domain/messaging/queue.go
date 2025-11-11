package messaging

import "context"

type Queue interface {
	Publish(ctx context.Context, queue string, message []byte) error
	PublishToTopic(ctx context.Context, topic string, key string, correlationID string, message []byte) error
	Consume(ctx context.Context, queue string, prefetchCount int, handler func(message []byte) error) error
	ConsumeFromTopic(ctx context.Context, topic string, key string, prefetchCount int, handler func(message []byte, exit chan struct{}) error) error
	Close()
}
