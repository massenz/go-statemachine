/*
 * Copyright (c) 2022 AlertAvert.com.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Author: Marco Massenzio (marco@alertavert.com)
 */

package main

import (
    "context"
    "flag"
    "fmt"
    "github.com/massenz/go-statemachine/api"
    protos "github.com/massenz/statemachine-proto/golang/api"
    "google.golang.org/grpc"
    "google.golang.org/protobuf/encoding/protojson"
    "strings"
    "time"
)

func main() {
    serverAddr := flag.String("addr", "localhost:7398",
        "The address (host:port) for the GRPC server")

    clientOptions := []grpc.DialOption{grpc.WithInsecure()}
    cc, _ := grpc.Dial(*serverAddr, clientOptions...)
    client := protos.NewStatemachineServiceClient(cc)

    start := time.Now()
    // Create a new Order tracked as an FSM
    putResponse, err := client.PutFiniteStateMachine(context.Background(),
        &protos.FiniteStateMachine{ConfigId: "test.orders:v3"})
    if err != nil {
        fmt.Printf("could not create FSM: %s\n", err)
        return
    }
    fmt.Println("Created FSM with ID:", putResponse.Id)
    fsm, err := protojson.Marshal(putResponse.Fsm)
    if err != nil {
        return
    }
    fmt.Println(string(fsm))

    // Fake order
    order := NewOrderDetails(putResponse.Id, "cust-1234", 123.55)

    for _, event := range []string{"accept", "ship", "foo", "deliver", "sign"} {
        if err := sendEvent(client, order, event); err != nil {
            fmt.Println("ERROR:", err)
            continue
        }
    }
    fmt.Println("Total time:", time.Since(start))
}

func sendEvent(client protos.StatemachineServiceClient, order *OrderDetails, event string) (err error) {
    // Once created, we want to `accept` the order
    evt := api.NewEvent(event)
    evt.Details = order.String()
    response, err := client.ProcessEvent(context.Background(),
        &protos.EventRequest{
            Event: evt,
            Dest:  strings.Join([]string{"test.orders", order.OrderId}, "#"),
        })
    if err != nil {
        fmt.Printf("Error: %s\n", err)
        return
    }
    evtId := response.GetEventId()
    fmt.Println("Event ID:", evtId)

    // Simulate a wait for the FSM to process the event
    time.Sleep(5 * time.Millisecond)

    outcome, err := client.GetEventOutcome(context.Background(), &protos.GetRequest{
        Id: strings.Join([]string{"test.orders", evtId}, "#"),
    })
    if err != nil {
        fmt.Println("Cannot get Outcome:", err)
        return
    }

    value, err := protojson.Marshal(outcome)
    if err != nil {
        fmt.Println(err)
        return
    }
    fmt.Println("Outcome:", string(value))
    return
}
