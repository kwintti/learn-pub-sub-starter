package main

import (
	"fmt"
	"log"
    //"reflect"

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
    armyMoveCh, _, err := pubsub.DeclareAndBind(conn, routing.ExchangePerilTopic, routing.ArmyMovesPrefix+"."+username, routing.ArmyMovesPrefix+".*", 1) 
    if err != nil {
        log.Fatalf("Couldn't declare and bind: %v", err)
    }
    defer armyMoveCh.Close()

    err = pubsub.SubscribeJSON(armyMoveCh, routing.ExchangePerilTopic, routing.ArmyMovesPrefix+"."+username, routing.ArmyMovesPrefix+".*", 1, handlerMove(gameState))
    if err != nil {
        log.Fatalf("Couldn't subscribe: %v", err)
    }
    

    for {
        input := gamelogic.GetInput()
        if len(input) == 0 {
            continue
        }
        switch input[0] {
        case "spawn":
            gameState.CommandSpawn(input)
        case "move": 
            log.Println(input)
            move, err := gameState.CommandMove(input)
            log.Println(move.Units)
            if err != nil {
                log.Fatalf("Couldn't move unit: %v", err)
            }
            err = pubsub.PublishJSON(armyMoveCh, routing.ExchangePerilTopic, routing.ArmyMovesPrefix+"."+username, move)
            if err != nil {
                log.Fatalf("Message not send: %v", err)
            }
            log.Println("Move published!")
        case "status":
            gameState.CommandStatus()
        case "help":
            gamelogic.PrintClientHelp()
        case "spam":
            log.Println("Spamming not allowed yet!") 
        case "quit":
            gamelogic.PrintQuit()
            return
        default:
            log.Println("Command not found!")
        }

    }

}

func handlerPause(gs *gamelogic.GameState) func(routing.PlayingState) string {
    return func(state routing.PlayingState) string {
        defer fmt.Print("> ")
        gs.HandlePause(state)
        return "Ack" 
    }
}

func handlerMove(gs *gamelogic.GameState) func(gamelogic.ArmyMove) string{
    return func(state gamelogic.ArmyMove) string{
        defer fmt.Print("> ")
        move := gs.HandleMove(state)
        if move == gamelogic.MoveOutComeSafe || move == gamelogic.MoveOutcomeMakeWar {
            return "Ack"
        }
        return "NackDiscard"
    }
}
