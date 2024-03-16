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
	"crypto/tls"
	"flag"
	"fmt"
	protos "github.com/massenz/statemachine-proto/golang/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/emptypb"
	"log"
	"time"
)

// Most basic binary to run health checks on the cmd.
// Used to assert readiness of the container/pod in Docker/Kubernetes.
func main() {
	var address = flag.String("host", "localhost:7398",
		"The address (host:port) for the GRPC cmd")
	var timeout = flag.Duration("timeout", 200*time.Millisecond,
		"timeout expressed as a duration string (e.g., 200ms, 1s, etc.)")
	var noTLS = flag.Bool("insecure", false, "disables TLS")
	flag.Parse()

	var creds credentials.TransportCredentials
	if *noTLS {
		creds = insecure.NewCredentials()
	} else {
		config := &tls.Config{
			InsecureSkipVerify: true,
		}
		creds = credentials.NewTLS(config)
	}

	cc, err := grpc.Dial(*address, grpc.WithTransportCredentials(creds))
	defer cc.Close()
	if err != nil {
		log.Fatalf("cannot open connection to %s: %v", *address, err)
	}

	client := protos.NewStatemachineServiceClient(cc)
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	resp, err := client.Health(ctx, &emptypb.Empty{})
	if err != nil {
		log.Fatal("cannot connect to cmd:", err)
	}
	jsonBytes, err := protojson.Marshal(resp)
	if err != nil {
		log.Fatal("Error while marshaling the message to JSON:", err)
	}
	fmt.Println(string(jsonBytes))
}
