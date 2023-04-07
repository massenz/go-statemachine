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
	"github.com/massenz/go-statemachine/api"
	"github.com/massenz/go-statemachine/grpc"
	protos "github.com/massenz/statemachine-proto/golang/api"
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
	cc, _ := g.Dial(address, g.WithTransportCredentials(creds))
	return &CliClient{protos.NewStatemachineServiceClient(cc)}
}

// sendEvent is an internal method that encapsulates sending the Event to the server,
// and wraps the StatemachineServiceClient.SendEvent function
func (c *CliClient) sendEvent(request *protos.EventRequest) (*protos.EventResponse, error) {
	api.UpdateEvent(request.Event)
	response, err := c.SendEvent(context.Background(), request)
	if err != nil {
		return nil, err
	}
	evtId := response.GetEventId()
	fmt.Println("Event ID:", evtId)

	var outcome *protos.EventResponse
	for remain := MaxRetries; remain > 0; remain-- {
		outcome, err = c.GetEventOutcome(context.Background(), &protos.EventRequest{
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
	data, _ := io.ReadAll(f)
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

// Get will retrieve the required entity from the server and generate the
// YAML representation accordingly.
// It takes two arguments, the kind and the id of the entity, and prints the
// contents returned by the server to stdout (or returns an error if not found)
func (c *CliClient) Get(kind, id string) error {
	return nil
}
