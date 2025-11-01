package messaging

import "context"

type Queue interface {
	Declare(queue string) error
	Publish(ctx context.Context, queue string, message []byte) error
	Consume(ctx context.Context, queue string, handler func(message []byte)) error
	Close()
}
