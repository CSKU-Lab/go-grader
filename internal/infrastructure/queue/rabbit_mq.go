package queue

import (
	"context"
	"errors"

	"github.com/CSKU-Lab/go-grader/domain/messaging"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

type rabbitmq struct {
	conn     *amqp.Connection
	ch       *amqp.Channel
	confirms chan amqp.Confirmation
	logger   *zap.SugaredLogger
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

	err = ch.Confirm(false)
	if err != nil {
		return nil, err
	}

	// we declare exchanges and bind queues here because producer doesn't need to know which queue to publish to
	// it just publishes to the exchange with the routing key, and RabbitMQ routes the message to the correct queue
	// so that we can decouple queue declaration from producers
	err = ch.ExchangeDeclare("grade", "direct", true, false, false, false, nil)
	if err != nil {
		return nil, err
	}

	err = ch.ExchangeDeclare("run", "direct", true, false, false, false, nil)
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

	err = ch.QueueBind("grade", "grade", "grade", false, nil)
	if err != nil {
		return nil, err
	}

	err = ch.QueueBind("run", "run", "run", false, nil)
	if err != nil {
		return nil, err
	}

	_, err = ch.QueueDeclare("run", true, false, false, false, nil)
	if err != nil {
		return nil, err
	}

	// topic exchanges for run results
	err = ch.ExchangeDeclare(
		"topic.run_results",
		"topic",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return nil, err
	}

	confirms := ch.NotifyPublish(make(chan amqp.Confirmation))

	return &rabbitmq{
		conn:     conn,
		ch:       ch,
		confirms: confirms,
		logger:   logger,
	}, nil
}

func (r *rabbitmq) Publish(ctx context.Context, exchange string, message []byte) error {
	err := r.ch.PublishWithContext(
		ctx,
		exchange,
		exchange,
		false,
		false,
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "application/json",
			Body:         message,
		},
	)
	if err != nil {
		return err
	}

	confirmed := <-r.confirms

	if confirmed.Ack {
		return nil
	} else {
		return errors.New("failed to publish message to the queue")
	}
}

func (r *rabbitmq) PublishToTopic(ctx context.Context, topic string, key string, correlationID string, message []byte) error {
	err := r.ch.PublishWithContext(
		ctx,
		topic,
		key,
		false,
		false,
		amqp.Publishing{
			ContentType:   "application/json",
			CorrelationId: correlationID,
			Body:          message,
		},
	)
	if err != nil {
		return err
	}

	confirmed := <-r.confirms

	if confirmed.Ack {
		return nil
	} else {
		return errors.New("failed to publish message to the queue")
	}
}

func (r *rabbitmq) ConsumeFromTopic(ctx context.Context, topic string, key string, prefetchCount int, handler func(message []byte, exit chan struct{}) error) error {
	q, err := r.ch.QueueDeclare("", false, true, true, false, nil)
	if err != nil {
		return err
	}

	err = r.ch.QueueBind(q.Name, key, topic, false, nil)
	if err != nil {
		return err
	}

	exitChan := make(chan struct{}, 1)

	msgs, err := r.ch.Consume(q.Name, "", true, true, false, false, nil)
	for {
		select {
		case <-exitChan:
			r.logger.Infoln("Exit signal received, stopping consuming messages from the queue")
			return nil
		case <-ctx.Done():
			r.logger.Errorln("Context has been doned", ctx.Err())
			return ctx.Err()
		case msg, ok := <-msgs:
			if !ok {
				r.logger.Infoln("Message channel closed, stopping consuming messages from the queue")
				return nil
			}

			if msg.MessageId != "" {
				r.logger.Infof("Message %s consumed", msg.MessageId)
			} else {
				r.logger.Info("Message consumed")
			}

			err := handler(msg.Body, exitChan)
			if err != nil {
				return err
			}
		}
	}
}

func (r *rabbitmq) Consume(ctx context.Context, queue string, prefetchCount int, handler func(message []byte) error) error {
	err := r.ch.Qos(prefetchCount, 0, false)
	if err != nil {
		return err
	}

	msgs, err := r.ch.ConsumeWithContext(
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

	for {
		select {
		case <-ctx.Done():
			r.logger.Info("Stopping consuming messages from the queue")
			return ctx.Err()
		case msg, ok := <-msgs:
			if !ok {
				r.logger.Info("Message channel closed, stopping consuming messages from the queue")
				return nil
			}
			go func() {
				if err := handler(msg.Body); err != nil {
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
		case err := <-errChan:
			return err
		}
	}
}

func (r *rabbitmq) Close() {
	r.ch.Close()
	r.conn.Close()
}
