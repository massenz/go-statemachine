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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"time"
)

func getStatusCode(response interface{}) codes.Code {
	// Get the gRPC status from the response
	s, ok := status.FromError(response.(error))
	if !ok {
		return codes.Unknown
	}

	// Return the status code
	return s.Code()
}

func main() {
	serverAddr := flag.String("addr", "localhost:7398",
		"The address (host:port) for the GRPC server")

	clientOptions := []grpc.DialOption{grpc.WithInsecure()}
	cc, _ := grpc.Dial(*serverAddr, clientOptions...)
	client := protos.NewStatemachineServiceClient(cc)

	start := time.Now()
	// Creates the new configuration, ignore error if it already exists
	var config protos.Configuration
	err := ReadConfig("data/orders.json", &config)
	if err != nil {
		fmt.Println("Could not parse configuration", err)
		return
	}
	response, err := client.PutConfiguration(context.Background(), &config)
	if err != nil {
		code := getStatusCode(err)
		if code == codes.AlreadyExists {
			fmt.Printf("Configuration `%s` exists\n", api.GetVersionId(&config))
		} else {
			fmt.Println("Could not store configuration: ", code.String())
			return
		}
	} else {
		fmt.Println("Orders configured: ", response.String())
	}

	req := &protos.PutFsmRequest{
		Fsm: &protos.FiniteStateMachine{ConfigId: api.GetVersionId(&config)},
	}
	// Create a new Order tracked as an FSM
	putResponse, err := client.PutFiniteStateMachine(context.Background(), req)
	if err != nil {
		fmt.Printf("could not create FSM: %s\n", err)
		return
	}
	fmt.Println("Created FSM with ID:", putResponse.GetId())
	fsm, err := protojson.Marshal(putResponse.GetFsm())
	if err != nil {
		return
	}
	fmt.Println(string(fsm))

	// Fake order
	order := NewOrderDetails(putResponse.Id, "cust-1234", 123.55)

	for _, event := range []string{"accept", "ship", "deliver", "sign"} {
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
	response, err := client.SendEvent(context.Background(),
		&protos.EventRequest{
			Event:  evt,
			Config: "test.orders",
			Id:     order.OrderId,
		})
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}
	evtId := response.GetEventId()
	fmt.Println("Event ID:", evtId)

	// Simulate a wait for the FSM to process the event
	time.Sleep(5 * time.Millisecond)

	outcome, err := client.GetEventOutcome(context.Background(), &protos.EventRequest{
		Config: "test.orders",
		Id:     evtId,
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
