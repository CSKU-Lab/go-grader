package messaging

import "context"

type Queue interface {
	Publish(ctx context.Context, queue string, message []byte) error
	Consume(ctx context.Context, queue string, handler func(message []byte)) error
	Close()
}
