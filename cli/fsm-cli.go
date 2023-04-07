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
	"fmt"
	. "github.com/massenz/go-statemachine/client"
	"google.golang.org/protobuf/types/known/emptypb"
	"os"
	"strings"
	"time"
)


var (
	// Release is set by the Makefile at build time
	Release string
)

func main() {
	var insecure = flag.Bool("insecure", false, "If set, TLS will be disabled (NOT recommended)")
	var serverAddr = flag.String("addr", "localhost:7398", "The address (host:port) for the GRPC server")

	flag.Parse()
	cmd := strings.ToLower(flag.Arg(0))
	if cmd == "" {
		// Nothing to do, print the version and exit
		fmt.Println("FSM CLI Client Rel.", Release)
		os.Exit(0)
	}

	c := NewClient(*serverAddr, !*insecure)
	r, err := c.Health(context.Background(), &emptypb.Empty{})
	if err != nil {
		fmt.Println("unhealthy server:", err)
		os.Exit(1)
	}
	fmt.Printf("Client %s connected to Server: %s at %s (%s)\n", Release, r.Release, *serverAddr, r.State)
	start := time.Now()

	switch cmd {
	case CmdSend:
		err = c.Send(flag.Arg(1))
	case CmdGet:
		err = c.Get(flag.Arg(1), flag.Arg(2))
	}
	if err != nil {
		fmt.Println("error:", err)
	}
	fmt.Printf("It took %v\n", time.Since(start))
}
