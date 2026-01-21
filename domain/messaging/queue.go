package messaging

import "context"

type Queue interface {
	Publish(ctx context.Context, exchange string, key string, derivery *Derivery) error
	Consume(ctx context.Context, queue string, prefetchCount int, handler func(derivery *Derivery, exit chan struct{}) error) error
	Close()
	CreateQueue(ctx context.Context, name string) (string, error)
}

type Derivery struct {
	Body          []byte
	CorrelationID string
	ReplyTo       string
}
