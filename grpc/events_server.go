/*
 * Copyright (c) 2022 AlertAvert.com.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Author: Marco Massenzio (marco@alertavert.com)
 */

package grpc

import (
    "context"
    "github.com/massenz/go-statemachine/api"
    "github.com/massenz/go-statemachine/pubsub"
    "github.com/massenz/slf4go/logging"
    "google.golang.org/grpc"
    "time"
)

type Config struct {
    EventsChannel chan<- pubsub.EventMessage
    Logger        *logging.Log
}

var _ api.EventsServer = (*grpcSubscriber)(nil)

type grpcSubscriber struct {
    api.UnimplementedEventsServer
    *Config
}

func newGrpcServer(config *Config) (srv *grpcSubscriber, err error) {
    srv = &grpcSubscriber{
        Config: config,
    }
    return srv, nil
}

func (s *grpcSubscriber) ConsumeEvent(ctx context.Context,
    request *api.EventRequest) (*api.EventResponse, error) {

    s.Logger.Trace("Received gRPC request: %v", request)
    evt := pubsub.EventMessage{
        Sender:         request.Event.Originator,
        Destination:    request.Dest,
        EventId:        request.Event.EventId,
        EventName:      request.Event.Transition.Event,
        EventTimestamp: time.Now(),
    }
    s.Logger.Trace("Sending Event to channel: %v", evt)
    // TODO: use the context to set a timeout and cancel the request if the channel cannot accept
    //       the event within the given timeout.
    s.EventsChannel <- evt
    return &api.EventResponse{Ok: true}, nil
}

func NewGrpcServer(config *Config) (*grpc.Server, error) {
    gsrv := grpc.NewServer()
    sub, err := newGrpcServer(config)
    if err != nil {
        return nil, err
    }
    api.RegisterEventsServer(gsrv, sub)
    return gsrv, nil
}
