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
    "fmt"
    "github.com/google/uuid"
    "github.com/massenz/slf4go/logging"
    "google.golang.org/grpc"
    "google.golang.org/protobuf/types/known/timestamppb"

    "github.com/massenz/go-statemachine/api"
    "github.com/massenz/go-statemachine/storage"

    protos "github.com/massenz/statemachine-proto/golang/api"
)

type Config struct {
    EventsChannel chan<- protos.EventRequest
    Store         storage.StoreManager
    Logger        *logging.Log
}

var _ protos.StatemachineServiceServer = (*grpcSubscriber)(nil)

type grpcSubscriber struct {
    protos.UnimplementedStatemachineServiceServer
    *Config
}

func newGrpcServer(config *Config) (srv *grpcSubscriber, err error) {
    srv = &grpcSubscriber{
        Config: config,
    }
    return srv, nil
}

func (s *grpcSubscriber) ConsumeEvent(ctx context.Context, request *protos.EventRequest) (*protos.EventResponse, error) {

    if request.Dest == "" {
        return nil, api.MissingDestinationError
    }
    if request.Event.Transition.Event == "" {
        return nil, api.MissingEventNameError
    }
    if request.Event.EventId == "" {
        request.Event.EventId = uuid.NewString()
    }
    if request.Event.Timestamp == nil {
        request.Event.Timestamp = timestamppb.Now()
    }
    s.Logger.Trace("Sending Event to channel: %v", request.Event)
    // TODO: use the context and cancel the request if the channel cannot accept
    //       the event within the given timeout.
    s.EventsChannel <- *request
    return &protos.EventResponse{EventId: request.Event.EventId}, nil
}

func (s *grpcSubscriber) PutConfiguration(ctx context.Context, cfg *protos.Configuration) (*protos.PutResponse, error) {
    // FIXME: use Context to set a timeout, etc.
    if err := api.CheckValid(cfg); err != nil {
        s.Logger.Error("invalid configuration: %v", err)
        return nil, err
    }
    if err := s.Store.PutConfig(cfg); err != nil {
        s.Logger.Error("could not store configuration: %v", err)
        return nil, err
    }
    s.Logger.Trace("configuration stored: %s", api.GetVersionId(cfg))
    return &protos.PutResponse{
        Id:     api.GetVersionId(cfg),
        Config: cfg,
        Fsm:    nil,
    }, nil
}

func (s *grpcSubscriber) GetConfiguration(ctx context.Context,
    request *protos.GetRequest) (*protos.Configuration, error) {
    s.Logger.Trace("retrieving Configuration %s", request.GetId())
    cfg, found := s.Store.GetConfig(request.GetId())
    if !found {
        return nil, fmt.Errorf("configuration %s not found", request.GetId())
    }
    return cfg, nil
}

func (s *grpcSubscriber) PutFiniteStateMachine(ctx context.Context,
    fsm *protos.FiniteStateMachine) (*protos.PutResponse, error) {
    // First check that the configuration for the FSM is valid
    _, ok := s.Store.GetConfig(fsm.ConfigId)
    if !ok {
        return nil, storage.ConfigNotFoundError
    }
    id := uuid.NewString()
    s.Logger.Trace("storing FSM [%s] configured with %s", id, fsm.ConfigId)
    if err := s.Store.PutStateMachine(id, fsm); err != nil {
        s.Logger.Error("could not store FSM [%v]: %v", fsm, err)
        return nil, err
    }
    return &protos.PutResponse{Id: id, Fsm: fsm}, nil
}

func (s *grpcSubscriber) GetFiniteStateMachine(ctx context.Context,
    request *protos.GetRequest) (*protos.FiniteStateMachine, error) {
    s.Logger.Trace("looking up FSM %s", request.GetId())
    fsm, ok := s.Store.GetStateMachine(request.GetId())
    if !ok {
        return nil, storage.FSMNotFoundError
    }
    return fsm, nil
}

func NewGrpcServer(config *Config) (*grpc.Server, error) {
    gsrv := grpc.NewServer()
    sub, err := newGrpcServer(config)
    if err != nil {
        return nil, err
    }
    protos.RegisterStatemachineServiceServer(gsrv, sub)
    return gsrv, nil
}
