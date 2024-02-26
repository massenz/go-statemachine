/*
 * Copyright (c) 2023 AlertAvert.com.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Author: Marco Massenzio (marco@alertavert.com)
 */

package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/golang/protobuf/ptypes/wrappers"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	g "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"gopkg.in/yaml.v3"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"github.com/massenz/go-statemachine/api"
	"github.com/massenz/go-statemachine/grpc"
	protos "github.com/massenz/statemachine-proto/golang/api"
)

const (
	MaxRetries             = 5
	IntervalBetweenRetries = 200 * time.Millisecond
)

func titleCase(s string) string {
	return cases.Title(language.English).String(s)
}

func NewClient(address string, hasTls bool) *CliClient {
	addr := strings.Split(address, ":")
	var creds credentials.TransportCredentials
	if !hasTls {
		fmt.Println("WARN: TLS Disabled")
		creds = insecure.NewCredentials()
	} else {
		clientTlsConfig := &tls.Config{}
		ca, err := grpc.ParseCAFile(path.Join(CertsDir, CaCert))
		if err != nil {
			panic(err)
		}
		clientTlsConfig.RootCAs = ca
		clientTlsConfig.ServerName = addr[0]
		creds = credentials.NewTLS(clientTlsConfig)
	}
	cc, err := g.Dial(address, g.WithTransportCredentials(creds))
	if err != nil {
		return nil
	}
	return &CliClient{protos.NewStatemachineServiceClient(cc)}
}

// sendEvent is an internal method that encapsulates sending the Event to the server,
// and wraps the StatemachineServiceClient.SendEvent function
func (c *CliClient) sendEvent(request *protos.EventRequest) (*protos.EventResponse, error) {
	api.UpdateEvent(request.Event)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	response, err := c.SendEvent(ctx, request)
	if err != nil {
		return nil, err
	}
	evtId := response.GetEventId()
	fmt.Println("Event ID:", evtId)

	var outcome *protos.EventResponse
	for remain := MaxRetries; remain > 0; remain-- {
		outcome, err = c.GetEventOutcome(ctx, &protos.EventRequest{
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

// Send processes CLI commands of the form `send config.yaml` by
// parsing the YAML according to its contents, dynamically adjusting the types.
// It only takes one argument, the path to the YAML file or `--` to use stdin
func (c *CliClient) Send(path string) error {
	var entity GenericEntity
	var f *os.File
	var err error

	if path == StdinFlag {
		f = os.Stdin
	} else {
		f, err = os.Open(path)
		if err != nil {
			return fmt.Errorf("cannot open %s: %v", path, err)
		}
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(data, &entity)
	if err != nil {
		return err
	}
	if entity.Version != Version {
		return fmt.Errorf("unsupported version %s", entity.Version)
	}

	handler, ok := SendHandlers[entity.Kind]
	if !ok {
		return fmt.Errorf("unknown Kind: %s", entity.Kind)
	}
	resp, grpcErr := handler(c, data)
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

// Get will retrieve the required entity from the cmd and generate the
// YAML representation accordingly.
// It takes two arguments, the kind and the id of the entity, and prints the
// contents returned by the server to stdout (or returns an error if not found)
func (c *CliClient) Get(kind, id string) error {

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	switch kind {
	case KindConfiguration:
		cfg, err := c.GetConfiguration(ctx, &wrappers.StringValue{Value: id})
		if err != nil {
			return err
		}
		data, err := yaml.Marshal(cfg)
		if err != nil {
			return err
		}
		fmt.Println(string(data))
	case KindFiniteStateMachine:
		parts := strings.Split(id, string(os.PathSeparator))
		if len(parts) != 2 {
			return fmt.Errorf("expected an FSM ID of the form `config-name/fsm-id`, got instead %s", id)
		}
		fsm, err := c.GetFiniteStateMachine(ctx, &protos.GetFsmRequest{
			Config: parts[0],
			Query:  &protos.GetFsmRequest_Id{Id: parts[1]},
		})
		if err != nil {
			return err
		}
		data, err := yaml.Marshal(fsm)
		if err != nil {
			return err
		}
		fmt.Println(string(data))
	default:
		return fmt.Errorf("kind `%s` unknown, please note they are case-sensitive (did you mean %s?)", kind,
			titleCase(kind))
	}

	return nil
}
