/*
 * Copyright (c) 2023 AlertAvert.com.  All rights reserved.
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
	"github.com/golang/protobuf/jsonpb"
	protos "github.com/massenz/statemachine-proto/golang/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"log"
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
	var address = flag.String("host", "localhost:7398",
		"The address (host:port) for the GRPC server")
	var timeout = flag.Duration("timeout", 200*time.Millisecond,
		"timeout expressed as a duration string (e.g., 200ms, 1s, etc.)")
	flag.Parse()
	cc, _ := grpc.Dial(*address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	client := protos.NewStatemachineServiceClient(cc)

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	start := time.Now()
	resp, err := client.Health(ctx, &emptypb.Empty{})
	if err != nil {
		log.Fatal("cannot connect to server:", err)
	}
	// Unmarshal your message to JSON format
	marshaler := &jsonpb.Marshaler{}
	jsonString, err := marshaler.MarshalToString(resp)
	if err != nil {
		log.Fatal("Error while marshaling the message to JSON:", err)
	}
	log.Println(jsonString)
	log.Println("Total time:", time.Since(start))
}
