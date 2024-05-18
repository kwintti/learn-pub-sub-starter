package main

import (
	"log"
	//"os"
	//"os/signal"
    "reflect"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)



func main() {
	log.Println("Starting Peril server...")
    url := "amqp://guest:guest@localhost:5672/"
    conn, err := amqp.Dial(url)
    if err != nil {
        log.Fatalf("Couldn't connect to the rabbit: %v", err)
    }
    defer conn.Close()
    log.Println("Connected to RabbitMQ")
    ch, err := conn.Channel()
    if err != nil {
        log.Fatalf("Couldn't create the channel: %v", err)
    }
    _, _, err = pubsub.DeclareAndBind(conn, routing.ExchangePerilTopic, "game_logs", routing.GameLogSlug, 0)
    if err != nil {
        log.Fatalf("Couldn't bind game_logs: %v", err)
    }

    err = pubsub.PublishJSON(ch, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{IsPaused: true})
    if err != nil {
        log.Fatalf("Message not send: %v", err)
    }
    log.Println("Pause message sent!")

    gamelogic.PrintServerHelp()

    for {
        input := gamelogic.GetInput()
        ok := reflect.DeepEqual(input, []string{"pause"})  
        if ok {
            err = pubsub.PublishJSON(ch, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{IsPaused: true})
            if err != nil {
                log.Fatalf("Message not send: %v", err)
            }
            log.Println("Pause message sent!")
        }
        ok = reflect.DeepEqual(input, []string{"resume"})  
        if ok {
            err = pubsub.PublishJSON(ch, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{IsPaused: false})
            if err != nil {
                log.Fatalf("Message not send: %v", err)
            }
            log.Println("Resume message sent!")
        }
        ok = reflect.DeepEqual(input, []string{"quit"})  
        if ok {
            break
        }
    }
    
    log.Println("Shutting down")


    //signalChan := make(chan os.Signal, 1)
    //signal.Notify(signalChan, os.Interrupt)
    //<- signalChan
    //log.Println("Shutting down")
}
