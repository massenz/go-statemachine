/*
 * Copyright (c) 2022 AlertAvert.com.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0
 * http://www.apache.org/licenses/LICENSE-2.0
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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"strings"
	"time"

	"github.com/massenz/go-statemachine/api"
	"github.com/massenz/go-statemachine/storage"
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
		if strings.Contains(err.Error(), "already exists") {
			return nil, status.Errorf(codes.AlreadyExists, "cannot store configuration: %v", err)
		}
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
	cfg, ok := s.Store.GetConfig(fsm.ConfigId)
	if !ok {
		return nil, status.Error(codes.FailedPrecondition, storage.NotFoundError(
			fsm.ConfigId).Error())
	}
	// FIXME: we need to allow clients to specify the ID of the FSM to create
	id := uuid.NewString()
	// If the State of the FSM is not specified,
	// we set it to the initial state of the configuration.
	if fsm.State == "" {
		fsm.State = cfg.StartingState
	}
	s.Logger.Trace("storing FSM [%s] configured with %s", id, fsm.ConfigId)
	if err := s.Store.PutStateMachine(id, fsm); err != nil {
		s.Logger.Error("could not store FSM [%v]: %v", fsm, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &protos.PutResponse{Id: id, Fsm: fsm}, nil
}

func (s *grpcSubscriber) GetFiniteStateMachine(ctx context.Context, request *protos.GetRequest) (
	*protos.FiniteStateMachine, error) {
	// TODO: use Context to set a timeout, and then pass it on to the Store.
	//       This may require a pretty large refactoring of the store interface.
	s.Logger.Debug("looking up FSM %s", request.GetId())
	// The ID in the request contains the FSM ID,
	// prefixed by the Config Name (which defines the "type" of FSM)
	splitId := strings.Split(request.GetId(), storage.KeyPrefixIDSeparator)
	if len(splitId) != 2 {
		return nil, status.Errorf(codes.InvalidArgument, "invalid FSM ID: %s", request.GetId())
	}
	fsm, ok := s.Store.GetStateMachine(splitId[1], splitId[0])
	if !ok {
		return nil, status.Error(codes.NotFound, storage.NotFoundError(request.GetId()).Error())
	}
	return fsm, nil
}

func (s *grpcSubscriber) GetEventOutcome(ctx context.Context, request *protos.GetRequest) (
	*protos.EventResponse, error) {

	s.Logger.Debug("looking up EventOutcome %s", request.GetId())
	dest := strings.Split(request.GetId(), "#")
	if len(dest) != 2 {
		return nil, status.Error(codes.InvalidArgument,
			fmt.Sprintf("invalid destination [%s] expected: <type>#<id>", request.GetId()))
	}
	smType, evtId := dest[0], dest[1]
	outcome, ok := s.Store.GetOutcomeForEvent(evtId, smType)
	if !ok {
		return nil, status.Error(codes.NotFound, fmt.Sprintf("outcome for event %s not found", evtId))
	}
	return &protos.EventResponse{
		EventId: evtId,
		Outcome: outcome,
	}, nil
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
