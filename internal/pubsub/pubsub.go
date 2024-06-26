package pubsub

import (
	"context"
	"encoding/json"
	"log"
    "encoding/gob"
    "bytes"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Acktype int


const (
    Ack Acktype = iota
    NackRequeue 
    NackDiscard
    
)

func PublishJSON[T any](ch *amqp.Channel, exchange, key string, val T) error {
    ctx := context.Background()
    b, err := json.Marshal(val)

    if err != nil {
        return err
    }

    return ch.PublishWithContext(ctx, exchange, key, false, false, amqp.Publishing{ContentType: "application/json", Body: b})

}

func PublishGob[T any](ch *amqp.Channel, exchange, key string, val T) error {
    ctx := context.Background()
    var b bytes.Buffer
    enc := gob.NewEncoder(&b) 
    err := enc.Encode(val)
    if err != nil {
        log.Println("Couldn't encode value")
        return err
    }

    return ch.PublishWithContext(ctx, exchange, key, false, false, amqp.Publishing{ContentType: "application/gob", Body: b.Bytes()})
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
    queue, err := ch.QueueDeclare(queueName, durable, autoDelete, exclusive, false, amqp.Table{"x-dead-letter-exchange": "peril_dlx"})
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
	handler func(T) Acktype,
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
            switch handler(toGen) {
            case Ack:
                err = msg.Ack(false)
                if err != nil {
                    log.Fatalf("Couldn't acknowledge msg: %v", err)
                }
            case NackDiscard:
                err = msg.Nack(false, false)
                if err != nil {
                    log.Fatalf("Couldn't acknowledge msg: %v", err)
                }
            case NackRequeue:
                err = msg.Nack(false, true)
                if err != nil {
                    log.Fatalf("Couldn't acknowledge msg: %v", err)
                }

            }
        }
    }()

    return nil

}

func SubscribeGob[T any](
	ch *amqp.Channel,
	exchange,
	queueName,
	key string,
	simpleQueueType int,
	handler func(T) Acktype,
) error {
    err := ch.Qos(10, 0, false)
    if err != nil {
        log.Fatalf("Couldn't set prefetch: %v", err)
    }
    messages, err := ch.Consume(queueName, "", false, false, false, false, nil)
    if err != nil {
        log.Fatalf("Couldn't consume: %v", err)
        return err
    }
    
    go func() { 

        var toGen T

        for msg := range messages {
            buff := bytes.NewBuffer(msg.Body)
            dec := gob.NewDecoder(buff)
            err := dec.Decode(&toGen)
            if err != nil {
                log.Fatalf("Couldn't unmarshal message: %v", err)
            }
            switch handler(toGen) {
            case Ack:
                err = msg.Ack(false)
                if err != nil {
                    log.Fatalf("Couldn't acknowledge msg: %v", err)
                }
            case NackDiscard:
                err = msg.Nack(false, false)
                if err != nil {
                    log.Fatalf("Couldn't acknowledge msg: %v", err)
                }
            case NackRequeue:
                err = msg.Nack(false, true)
                if err != nil {
                    log.Fatalf("Couldn't acknowledge msg: %v", err)
                }

            }
        }
    }()

    return nil

}
