package queue

import (
	"context"
	"errors"

	"github.com/CSKU-Lab/go-grader/domain/constants"
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

	confirms := ch.NotifyPublish(make(chan amqp.Confirmation))

	return &rabbitmq{
		conn:     conn,
		ch:       ch,
		confirms: confirms,
		logger:   logger,
	}, nil
}

func (r *rabbitmq) declareQueue(queue string) (amqp.Queue, error) {
	return r.ch.QueueDeclare(
		queue,
		true,
		false,
		false,
		false,
		nil,
	)
}

func (r *rabbitmq) Publish(ctx context.Context, queue string, message []byte) error {
	q, err := r.declareQueue(queue)
	if err != nil {
		return err
	}

	err = r.ch.PublishWithContext(
		ctx,
		"",
		q.Name,
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

func (r *rabbitmq) Consume(ctx context.Context, queue string, handler func(message []byte)) error {
	q, err := r.declareQueue(queue)
	if err != nil {
		return err
	}

	err = r.ch.Qos(constants.MAX_QUEUES, 0, false)
	if err != nil {
		return err
	}

	msgs, err := r.ch.ConsumeWithContext(
		ctx,
		q.Name,
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

	for msg := range msgs {
		go func() {
			handler(msg.Body)
			msg.Ack(false)
			if msg.MessageId != "" {
				r.logger.Infof("Message %s consumed", msg.MessageId)
			} else {
				r.logger.Info("Message consumed")
			}
		}()
	}

	return nil
}

func (r *rabbitmq) Close() {
	r.ch.Close()
	r.conn.Close()
}
