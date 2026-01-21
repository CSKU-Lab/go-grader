package queue

import (
	"context"
	"errors"

	"github.com/CSKU-Lab/go-grader/domain/messaging"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

type rabbitmq struct {
	conn   *amqp.Connection
	logger *zap.SugaredLogger
}

func NewRabbitMQ(logger *zap.SugaredLogger, connStr string) (messaging.Queue, error) {
	conn, err := amqp.Dial(connStr)
	if err != nil {
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, err
	}

	_, err = ch.QueueDeclare("grade", true, false, false, false, nil)
	if err != nil {
		return nil, err
	}

	_, err = ch.QueueDeclare("run", true, false, false, false, nil)
	if err != nil {
		return nil, err
	}

	return &rabbitmq{
		conn:   conn,
		logger: logger,
	}, nil
}

func (r *rabbitmq) CreateQueue(ctx context.Context, name string) (string, error) {
	ch, err := r.conn.Channel()
	if err != nil {
		return "", err
	}

	q, err := ch.QueueDeclare(name, false, true, true, false, nil)
	if err != nil {
		return "", err
	}

	return q.Name, nil
}

func (r *rabbitmq) Publish(ctx context.Context, exchange string, key string, derivery *messaging.Derivery) error {
	ch, err := r.conn.Channel()
	if err != nil {
		return err
	}

	err = ch.Confirm(false)
	if err != nil {
		return err
	}

	err = ch.PublishWithContext(
		ctx,
		exchange,
		key,
		false,
		false,
		amqp.Publishing{
			ContentType:   "application/json",
			CorrelationId: derivery.CorrelationID,
			ReplyTo:       derivery.ReplyTo,
			Body:          derivery.Body,
		},
	)
	if err != nil {
		return err
	}

	confirmed := <-ch.NotifyPublish(make(chan amqp.Confirmation))

	if confirmed.Ack {
		return nil
	} else {
		return errors.New("failed to publish message to the queue")
	}
}

func (r *rabbitmq) Consume(ctx context.Context, queue string, prefetchCount int, handler func(derivery *messaging.Derivery, exit chan struct{}) error) error {
	ch, err := r.conn.Channel()
	if err != nil {
		return err
	}

	err = ch.Qos(prefetchCount, 0, false)
	if err != nil {
		return err
	}

	msgs, err := ch.ConsumeWithContext(
		ctx,
		queue,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	errChan := make(chan error, 1)
	exitChan := make(chan struct{}, 1)

	for {
		select {
		case <-ctx.Done():
			r.logger.Info("Stopping consuming messages from the queue")
			return ctx.Err()
		case err := <-errChan:
			return err
		case <-exitChan:
			r.logger.Info("Exit signal received, stopping consuming messages from the queue")
			return nil
		case msg, ok := <-msgs:
			r.logger.Info("Message received from the queue")
			if !ok {
				r.logger.Info("Message channel closed, stopping consuming messages from the queue")
				return nil
			}
			go func() {
				derivery := &messaging.Derivery{
					Body:          msg.Body,
					CorrelationID: msg.CorrelationId,
					ReplyTo:       msg.ReplyTo,
				}

				if err := handler(derivery, exitChan); err != nil {
					errChan <- err
					msg.Nack(false, true)
					return
				}

				if err = msg.Ack(false); err != nil {
					errChan <- err
					return
				}

				if msg.MessageId != "" {
					r.logger.Infof("Message %s consumed", msg.MessageId)
				} else {
					r.logger.Info("Message consumed")
				}
			}()
		}
	}
}

func (r *rabbitmq) Close() {
	r.conn.Close()
}
