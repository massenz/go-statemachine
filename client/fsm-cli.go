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
	"github.com/massenz/go-statemachine/api"
	"github.com/massenz/go-statemachine/grpc"
	protos "github.com/massenz/statemachine-proto/golang/api"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	g "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
	"gopkg.in/yaml.v3"
	"io"
	"os"
	"strings"
	"time"
)

const (
	MaxRetries             = 5
	IntervalBetweenRetries = 200 * time.Millisecond
)

var (
	client protos.StatemachineServiceClient
	// Release is set by the Makefile at build time
	Release string
)

func titleCase(s string) string {
	return cases.Title(language.English).String(s)
}
func NewClient(address string, hasTls bool) protos.StatemachineServiceClient {
	addr := strings.Split(address, ":")
	var creds credentials.TransportCredentials
	if !hasTls {
		fmt.Println("WARN: TLS Disabled")
		creds = insecure.NewCredentials()
	} else {
		clientTlsConfig := &tls.Config{}
		ca, err := grpc.ParseCAFile("certs/ca.pem")
		if err != nil {
			panic(err)
		}
		clientTlsConfig.RootCAs = ca
		clientTlsConfig.ServerName = addr[0]
		creds = credentials.NewTLS(clientTlsConfig)
	}
	cc, _ := g.Dial(address, g.WithTransportCredentials(creds))
	return protos.NewStatemachineServiceClient(cc)
}

type ExecFunc = func([]string) error

var router = map[string]ExecFunc{
	CmdGet:  Get,
	CmdSend: Send,
}

// Send processes CLI commands of the form `send config.yaml` by
// parsing the YAML according to its contents, dynamically adjusting the types.
// It only takes one argument, the path to the YAML file or `--` to use stdin
func Send(flags []string) error {
	var entity GenericEntity
	var resp interface{}
	var grpcErr error

	if len(flags) != 1 {
		return fmt.Errorf("unexpected arguments: %v", flags)
	}
	path := flags[0]
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("cannot open %s: %v", path, err)
	}
	data, _ := io.ReadAll(f)
	err = yaml.Unmarshal(data, &entity)
	if err != nil {
		return err
	}
	if entity.Version != Version {
		return fmt.Errorf("unsupported version %s", entity.Version)
	}

	switch entity.Kind {
	case KindConfiguration:
		var c ConfigEntity
		err = yaml.Unmarshal(data, &c)
		if err != nil {
			return err
		}
		resp, grpcErr = client.PutConfiguration(context.Background(), c.Spec)
	case KindFiniteStateMachine:
		var fsm FsmEntity
		err = yaml.Unmarshal(data, &fsm)
		if err != nil {
			return err
		}
		request := &protos.PutFsmRequest{Id: entity.Id, Fsm: fsm.Spec}
		resp, grpcErr = client.PutFiniteStateMachine(context.Background(), request)
	case KindEvent:
		var evt EventRequestEntity
		err = yaml.Unmarshal(data, &evt)
		if err != nil {
			return err
		}
		resp, grpcErr = SendEvent(evt.Spec)
	default:
		return fmt.Errorf("unknown Kind: %s", entity.Kind)
	}
	if grpcErr != nil {
		code := getStatusCode(grpcErr)
		if code == codes.AlreadyExists {
			fmt.Printf("entity `%s` exists\n", entity.Kind)
		}
		return grpcErr
	}
	out, err := yaml.Marshal(resp)
	if err == nil {
		fmt.Printf("Result:\n%v", string(out))
		resp, ok := resp.(*protos.EventResponse)
		if ok {
			// We can provide a bit more granularity in this case
			fmt.Printf("Outcome.Code: %s\n", resp.Outcome.Code.String())
		}
	}

	return err
}

// Get will retrieve the required entity from the server and generate the
// YAML representation accordingly.
// It takes two arguments, the Kind and the id of the entity, and prints the
// contents returned by the server to stdout (or returns an error if not found)
func Get(flags []string) error {
	return nil
}

func main() {
	var insecure = flag.Bool("insecure", false, "If set, TLS will be disabled (NOT recommended)")
	var serverAddr = flag.String("addr", "localhost:7398", "The address (host:port) for the GRPC server")

	flag.Parse()
	client = NewClient(*serverAddr, !*insecure)
	r, err := client.Health(context.Background(), &emptypb.Empty{})
	if err != nil {
		fmt.Println("unhealthy server:", err)
		os.Exit(1)
	}
	fmt.Printf("Client %s connected to Server: %s at %s (%s)\n", Release, r.Release, *serverAddr, r.State)
	cmd := strings.ToLower(flag.Arg(0))
	if cmd == "" {
		fmt.Println("nothing to do, exit")
		os.Exit(0)
	}
	start := time.Now()

	err = router[cmd](flag.Args()[1:])
	if err != nil {
		fmt.Println("error:", err)
	}
	fmt.Printf("It took %v\n", time.Since(start))
}

func SendEvent(request *protos.EventRequest) (*protos.EventResponse, error) {
	api.UpdateEvent(request.Event)
	response, err := client.SendEvent(context.Background(), request)
	if err != nil {
		return nil, err
	}
	evtId := response.GetEventId()
	fmt.Println("Event ID:", evtId)

	var outcome *protos.EventResponse
	for remain := MaxRetries; remain > 0; remain-- {
		outcome, err = client.GetEventOutcome(context.Background(), &protos.EventRequest{
			Config: request.Config,
			Id:     evtId,
		})
		if err != nil && getStatusCode(err) == codes.NotFound {
			// It may take a beat to get the outcome stored in Redis
			time.Sleep(IntervalBetweenRetries)
			continue
		}
		break
	}
	return outcome, nil
}
