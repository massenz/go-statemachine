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

	c := NewClient(*serverAddr, !*insecure)
	if c == nil {
		fmt.Printf("cannot connect to server at %s\n", *serverAddr)
		os.Exit(1)
	}
	r, err := c.Health(context.Background(), &emptypb.Empty{})
	if err != nil {
		fmt.Printf("server at %s is in an unhealthy state: %v\n", *serverAddr, err)
		os.Exit(2)
	}

	switch cmd {
	case CmdSend:
		err = c.Send(flag.Arg(1))
	case CmdGet:
		err = c.Get(flag.Arg(1), flag.Arg(2))
	case CmdVersion:
		fmt.Println("FSM CLI Client Rel.", Release)
		fmt.Printf("Connected to Server: %s at %s (%s)\n", r.Release, *serverAddr, r.State)
		// Nothing else to do, just exit
		os.Exit(0)
	default:
		fmt.Printf("unknown or missing command `%s`\n", cmd)
		os.Exit(3)
	}
	if err != nil {
		fmt.Printf("cannot execute command `%s`: %v\n", cmd, err)
		os.Exit(1)
	}
}
