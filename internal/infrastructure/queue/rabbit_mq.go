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

	// we declare exchanges and bind queues here because producer doesn't need to know which queue to publish to
	// it just publishes to the exchange with the routing key, and RabbitMQ routes the message to the correct queue
	// so that we can decouple queue declaration from producers
	err = ch.ExchangeDeclare("grade", "direct", true, false, false, false, nil)
	if err != nil {
		return nil, err
	}

	err = ch.ExchangeDeclare("grade_results", "direct", true, false, false, false, nil)
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

	_, err = ch.QueueDeclare("grade_results", true, false, false, false, nil)
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

	err = ch.QueueBind("grade_results", "grade_results", "grade_results", false, nil)
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
		"run_results",
		"direct",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return nil, err
	}

	return &rabbitmq{
		conn:   conn,
		logger: logger,
	}, nil
}

func (r *rabbitmq) Publish(ctx context.Context, topic string, key string, correlationID string, message []byte) error {
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

	confirmed := <-ch.NotifyPublish(make(chan amqp.Confirmation))

	if confirmed.Ack {
		return nil
	} else {
		return errors.New("failed to publish message to the queue")
	}
}

func (r *rabbitmq) ConsumeFromTopic(ctx context.Context, topic string, key string, prefetchCount int, handler func(message []byte, exit chan struct{}) error) error {
	ch, err := r.conn.Channel()
	if err != nil {
		return err
	}

	q, err := ch.QueueDeclare("", false, true, true, false, nil)
	if err != nil {
		return err
	}

	err = ch.QueueBind(q.Name, key, topic, false, nil)
	if err != nil {
		return err
	}

	exitChan := make(chan struct{}, 1)
	consumerTag, err := ch.Consume(q.Name, "", true, true, false, false, nil)
	if err != nil {
		return err
	}

	defer func() {
		close(exitChan)
		ch.Cancel(q.Name, false)
		ch.QueueDelete(q.Name, false, false, false)
	}()

	for {
		select {
		case <-exitChan:
			r.logger.Infoln("Exit signal received, stopping consuming messages from the queue")
			return nil
		case <-ctx.Done():
			r.logger.Errorln("Context has been doned", ctx.Err())
			return ctx.Err()
		case msg, ok := <-consumerTag:
			if !ok {
				r.logger.Infoln("Message channel closed, stopping consuming messages from the queue")
				return nil
			}

			select {
			case <-exitChan:
				r.logger.Infoln("Exit signal received, stopping consuming messages from the queue")
				msg.Nack(false, true)
				return nil
			default:
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
	r.conn.Close()
}
