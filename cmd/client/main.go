package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

type ChannelStruct struct {
    ChannelTopic *amqp.Channel
}

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
    chWar, err := conn.Channel()
    if err != nil {
        log.Fatalf("Error creating channel: %v", err)
    }
    defer chWar.Close()
    go handlerWar(gameState, conn)
    

    err = pubsub.SubscribeJSON(armyMoveCh, routing.ExchangePerilTopic, routing.ArmyMovesPrefix+"."+username, routing.ArmyMovesPrefix+".*", 1, handlerMove(gameState, chWar))
    if err != nil {
        log.Fatalf("Couldn't subscribe: %v", err)
    }

    chSpam, err := conn.Channel()
    if err != nil {
        log.Fatalf("Couldn't connect to spam channel: %v", err)
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
            move, err := gameState.CommandMove(input)
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
            n, err := strconv.Atoi(input[1])
            if err != nil {
                log.Fatalf("Couldn't parse number: %v", err)
            }
            for i := 0; i < n; i++ {
                spamMsg := gamelogic.GetMaliciousLog()
                pubsub.PublishGob(chSpam, routing.ExchangePerilTopic, routing.GameLogSlug+"."+username, 
                routing.GameLog{CurrentTime: time.Now(), Message: spamMsg, Username: username})
                
            }
        case "quit":
            gamelogic.PrintQuit()
            return
        default:
            log.Println("Command not found!")
        }

    }

}

func handlerPause(gs *gamelogic.GameState) func(routing.PlayingState) pubsub.Acktype {
    return func(state routing.PlayingState) pubsub.Acktype {
        defer fmt.Print("> ")
        gs.HandlePause(state)
        return pubsub.Ack 
    }
}

func handlerMove(gs *gamelogic.GameState, ch *amqp.Channel) func(gamelogic.ArmyMove) pubsub.Acktype{
    return func(move gamelogic.ArmyMove) pubsub.Acktype{
        defer fmt.Print("> ")
        outcome := gs.HandleMove(move)
        if outcome == gamelogic.MoveOutcomeMakeWar {
            err := pubsub.PublishJSON(ch, routing.ExchangePerilTopic, routing.WarRecognitionsPrefix+"."+gs.GetUsername(), 
            gamelogic.RecognitionOfWar{
                Attacker: move.Player, 
                Defender: gs.GetPlayerSnap(),})
            if err != nil {
                log.Fatalf("Message not send: %v", err)
                return pubsub.NackRequeue 
            }
            return pubsub.Ack
        }
        if outcome == gamelogic.MoveOutComeSafe {
            return pubsub.Ack 
        }
        return pubsub.NackDiscard
    }
}


func handlerWar(gs *gamelogic.GameState, conn *amqp.Connection) {
        ch, _, err := pubsub.DeclareAndBind(conn, routing.ExchangePerilTopic, routing.WarRecognitionsPrefix, routing.WarRecognitionsPrefix+".*", 0) 
        if err != nil {
            log.Fatalf("Couldn't bind: %v", err)
        }
        warMessages, err := ch.Consume(routing.WarRecognitionsPrefix, "", false, false, false, false, nil)
        if err != nil {
            log.Fatalf("Couldn't start consuming messages: %v", err)
        }

        chLog, err := conn.Channel()
        if err != nil {
            log.Fatalf("Couldn't subscibe to log channel: %v", err)
        }


        go func() {

            var msgBody gamelogic.RecognitionOfWar 
            var message string

            for msg := range warMessages {
                err := json.Unmarshal(msg.Body, &msgBody)
                if err != nil {
                    log.Fatalf("Couldn't unmarshal message: %v", err)
                }
                outcome, winner, loser := gs.HandleWar(msgBody)
                switch outcome {
                case gamelogic.WarOutcomeNotInvolved:
                    msg.Nack(false, true)
                case gamelogic.WarOutcomeNoUnits:
                    msg.Nack(false, false)
                case gamelogic.WarOutcomeOpponentWon:
                    msg.Ack(true)
                    message = winner + " won a war agains " + loser
                case gamelogic.WarOutcomeYouWon:
                    msg.Ack(true)
                    message = winner + " won a war agains " + loser
                case gamelogic.WarOutcomeDraw:
                    msg.Ack(true)
                    message = "A war between " + winner + " and " + loser + "resulted in a draw" 
                }
                fmt.Print("> ")

                err = pubsub.PublishGob(chLog, routing.ExchangePerilTopic, routing.GameLogSlug+"."+gs.GetUsername(),
                routing.GameLog{CurrentTime: time.Now(),
                                Message: message,
                                Username: gs.GetUsername(),})
                if err != nil {
                    log.Fatalf("Couldn't publish: %v", err)
                    msg.Nack(false, true)
                }
            }

    }()
}
