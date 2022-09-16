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
	"github.com/google/uuid"
	"github.com/massenz/go-statemachine/api"
	"github.com/massenz/go-statemachine/storage"
	"github.com/massenz/slf4go/logging"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"time"

	protos "github.com/massenz/statemachine-proto/golang/api"
)

type Config struct {
	EventsChannel chan<- protos.EventRequest
	Store         storage.StoreManager
	Logger        *logging.Log
	Timeout       time.Duration
}

var _ protos.StatemachineServiceServer = (*grpcSubscriber)(nil)

const (
	DefaultTimeout = 200 * time.Millisecond
)

type grpcSubscriber struct {
	protos.UnimplementedStatemachineServiceServer
	*Config
}

func (s *grpcSubscriber) ProcessEvent(ctx context.Context, request *protos.EventRequest) (*protos.
	EventResponse, error) {
	if request.Dest == "" {
		return nil, status.Error(codes.FailedPrecondition, api.MissingDestinationError.Error())
	}
	if request.GetEvent() == nil || request.Event.GetTransition() == nil ||
		request.Event.Transition.GetEvent() == "" {
		return nil, status.Error(codes.FailedPrecondition, api.MissingEventNameError.Error())
	}
	// If missing, add ID and timestamp.
	api.UpdateEvent(request.Event)

	var timeout = s.Timeout
	deadline, hasDeadline := ctx.Deadline()
	if hasDeadline {
		timeout = deadline.Sub(time.Now())
	}
	s.Logger.Trace("Sending Event to channel: %v", request.Event)
	select {
	case s.EventsChannel <- *request:
		return &protos.EventResponse{
			EventId: request.Event.EventId,
		}, nil
	case <-time.After(timeout):
		s.Logger.Error("Timeout exceeded when trying to post event to internal channel")
		return nil, status.Error(codes.DeadlineExceeded, "cannot post event")
	}
}

func (s *grpcSubscriber) PutConfiguration(ctx context.Context, cfg *protos.Configuration) (*protos.PutResponse, error) {
	// FIXME: use Context to set a timeout, etc.
	if err := api.CheckValid(cfg); err != nil {
		s.Logger.Error("invalid configuration: %v", err)
		return nil, status.Errorf(codes.InvalidArgument, "invalid configuration: %v", err)
	}
	if err := s.Store.PutConfig(cfg); err != nil {
		s.Logger.Error("could not store configuration: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	s.Logger.Trace("configuration stored: %s", api.GetVersionId(cfg))
	return &protos.PutResponse{
		Id:     api.GetVersionId(cfg),
		Config: cfg,
	}, nil
}

func (s *grpcSubscriber) GetConfiguration(ctx context.Context, request *protos.GetRequest) (
	*protos.Configuration, error) {
	s.Logger.Trace("retrieving Configuration %s", request.GetId())
	cfg, found := s.Store.GetConfig(request.GetId())
	if !found {
		return nil, status.Errorf(codes.NotFound, "configuration %s not found", request.GetId())
	}
	return cfg, nil
}

func (s *grpcSubscriber) PutFiniteStateMachine(ctx context.Context,
	fsm *protos.FiniteStateMachine) (*protos.PutResponse, error) {
	// First check that the configuration for the FSM is valid
	_, ok := s.Store.GetConfig(fsm.ConfigId)
	if !ok {
		return nil, status.Error(codes.FailedPrecondition, storage.ConfigNotFoundError.Error())
	}
	id := uuid.NewString()
	s.Logger.Trace("storing FSM [%s] configured with %s", id, fsm.ConfigId)
	if err := s.Store.PutStateMachine(id, fsm); err != nil {
		s.Logger.Error("could not store FSM [%v]: %v", fsm, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &protos.PutResponse{Id: id, Fsm: fsm}, nil
}

func (s *grpcSubscriber) GetFiniteStateMachine(ctx context.Context,
	request *protos.GetRequest) (*protos.FiniteStateMachine, error) {
	s.Logger.Trace("looking up FSM %s", request.GetId())
	fsm, ok := s.Store.GetStateMachine(request.GetId())
	if !ok {
		return nil, status.Error(codes.NotFound, storage.FSMNotFoundError.Error())
	}
	return fsm, nil
}

// NewGrpcServer creates a new gRPC server to handle incoming events and other API calls.
// The `Config` can be used to configure the backing store, a timeout and the logger.
func NewGrpcServer(config *Config) (*grpc.Server, error) {
	// Unless explicitly configured, we use for the server the same timeout as for the Redis store
	if config.Timeout == 0 {
		config.Timeout = DefaultTimeout
	}
	gsrv := grpc.NewServer()
	protos.RegisterStatemachineServiceServer(gsrv, &grpcSubscriber{Config: config})
	return gsrv, nil
}
