package pubsub

import (
	"context"
	"encoding/json"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

func PublishJSON[T any](ch *amqp.Channel, exchange, key string, val T) error {
    ctx := context.Background()
    b, err := json.Marshal(val)

    if err != nil {
        return err
    }

    return ch.PublishWithContext(ctx, exchange, key, false, false, amqp.Publishing{ContentType: "application/json", Body: b})

}

func DeclareAndBind(
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
    simpleQueueType int, // an enum to represent "durable" or 1:"transient"
) (*amqp.Channel, amqp.Queue, error) {
    ch, err := conn.Channel()
    if err != nil {
        return nil, amqp.Queue{}, err
    }

    durable := true
    autoDelete := false
    exclusive := false
    
    if simpleQueueType == 1 {
        durable = false
        autoDelete = true
        exclusive = true
    }
    queue, err := ch.QueueDeclare(queueName, durable, autoDelete, exclusive, false, nil)
    if err != nil {
        return nil, amqp.Queue{}, err
    }
    err = ch.QueueBind(queueName, key, exchange, false, nil)
    if err != nil {
        return nil, amqp.Queue{}, err
    }

    return ch, queue, nil
}


func SubscribeJSON[T any](
	ch *amqp.Channel,
	exchange,
	queueName,
	key string,
	simpleQueueType int,
	handler func(T),
) error {
    messages, err := ch.Consume(queueName, "", false, false, false, false, nil)
    if err != nil {
        log.Fatalf("Couldn't consume: %v", err)
        return err
    }
    
    go func() { 

        var toGen T

        for msg := range messages {
            err := json.Unmarshal(msg.Body, &toGen)
            if err != nil {
                log.Fatalf("Couldn't unmarshal message: %v", err)
            }
            handler(toGen)
            err = msg.Ack(false)
            if err != nil {
                log.Fatalf("Couldn't acknowledge msg: %v", err)
            }
        }
    }()

    return nil

}
