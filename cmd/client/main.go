package main

import (
	"fmt"
	"log"
    "reflect"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	fmt.Println("Starting Peril client...")
    url := "amqp://guest:guest@localhost:5672/"
    conn, err := amqp.Dial(url)
    if err != nil {
        log.Fatalf("Couldn't establish connection: %v", err)
    }
    defer conn.Close()
    username, err := gamelogic.ClientWelcome() 
    if err != nil {
        log.Fatalf("Couldnt't create user: %v", err)
    }
    ch, _, err := pubsub.DeclareAndBind(conn, routing.ExchangePerilDirect, routing.PauseKey+"."+username, routing.PauseKey, 1) 
    if err != nil {
        log.Fatalf("Couldn't declare and bind: %v", err)
    }
    defer ch.Close()

    gameState := gamelogic.NewGameState(username)
    err = pubsub.SubscribeJSON(ch, routing.ExchangePerilDirect, routing.PauseKey+"."+username, routing.PauseKey, 1, handlerPause(gameState))
    if err != nil {
        log.Fatalf("Couldn't subscribe: %v", err)
    }

    for {
        input := gamelogic.GetInput() 
        if len(input) != 0 {
        ok := reflect.DeepEqual(input[0], "spawn")
        if ok {
            gameState.CommandSpawn(input) 
            continue
        }
        ok = reflect.DeepEqual(input[0], "move")
        if ok {
            gameState.CommandMove(input) 
            continue
        }
        ok = reflect.DeepEqual(input[0], "status")
        if ok {
            gameState.CommandStatus() 
            continue
        }
        ok = reflect.DeepEqual(input[0], "help")
        if ok {
            gamelogic.PrintClientHelp() 
            continue
        }
        ok = reflect.DeepEqual(input[0], "spam")
        if ok {
            log.Println("Spamming not allowed yet!") 
            continue
        }
        ok = reflect.DeepEqual(input[0], "quit")
        if ok {
            gamelogic.PrintQuit() 
            break
        }
        log.Println("Command not found!")

    }

    }

}

func handlerPause(gs *gamelogic.GameState) func(routing.PlayingState) {
    return func(state routing.PlayingState) {
        defer fmt.Print("> ")
        gs.HandlePause(state)
    }
}
