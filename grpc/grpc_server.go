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
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/google/uuid"
	"github.com/massenz/slf4go/logging"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/massenz/go-statemachine/api"
	"github.com/massenz/go-statemachine/internal/config"
	"github.com/massenz/go-statemachine/storage"

	protos "github.com/massenz/statemachine-proto/golang/api"
)

type Config struct {
	EventsChannel chan<- protos.EventRequest
	Store         storage.StoreManager
	Logger        *logging.Log
	Timeout       time.Duration
	ServerAddress string
	TlsEnabled    bool
	TlsCerts      string
	TlsMutual     bool
}

type StatemachineStream = protos.StatemachineService_StreamAllInstateServer
type ConfigurationStream = protos.StatemachineService_StreamAllConfigurationsServer

var _ protos.StatemachineServiceServer = (*grpcSubscriber)(nil)

const (
	DefaultTimeout = 200 * time.Millisecond
)

type grpcSubscriber struct {
	protos.UnimplementedStatemachineServiceServer
	*Config
}

// Health will return the status of the server and the underlying store
func (s *grpcSubscriber) Health(context.Context, *emptypb.Empty) (*protos.HealthResponse, error) {
	var response = &protos.HealthResponse{
		State:      protos.HealthResponse_READY,
		Release:    api.Release,
		TlsEnabled: s.TlsEnabled,
	}
	if s.Store == nil {
		s.Logger.Error("Redis store not initialized, cannot process requests")
		return nil, status.Error(codes.Internal, "data store not configured")
	}
	if s.EventsChannel == nil {
		s.Logger.Error("events channel not initialized, cannot process events")
		response.State = protos.HealthResponse_NOT_READY
		return response, nil
	}
	err := s.Store.Health()
	if err != nil {
		s.Logger.Error("Redis store not ready: %v", err)
		response.State = protos.HealthResponse_NOT_READY
	}
	return response, nil
}

func (s *grpcSubscriber) SendEvent(ctx context.Context, request *protos.EventRequest) (*protos.
	EventResponse, error) {
	if request.GetId() == "" {
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
	if err := api.CheckValid(cfg); err != nil {
		s.Logger.Error("invalid configuration: %v", err)
		return nil, status.Errorf(codes.InvalidArgument, "invalid configuration: %v", err)
	}
	if deadline, ok := ctx.Deadline(); ok {
		if deadline.Before(time.Now()) {
			return nil, ctx.Err()
		}
	}
	if err := s.Store.PutConfig(cfg); err != nil {
		s.Logger.Error("could not store configuration: %v", err)
		if strings.Contains(err.Error(), "already exists") {
			return nil, status.Errorf(codes.AlreadyExists, "cannot store configuration: %v", err)
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	if deadline, ok := ctx.Deadline(); ok {
		if deadline.Before(time.Now()) {
			return nil, ctx.Err()
		}
	}
	s.Logger.Trace("configuration stored: %s", api.GetVersionId(cfg))
	return &protos.PutResponse{
		Id: api.GetVersionId(cfg),
		// Note: this is the magic incantation to use a `one_of` field in Protobuf.
		EntityResponse: &protos.PutResponse_Config{Config: cfg},
	}, nil
}
func (s *grpcSubscriber) GetAllConfigurations(ctx context.Context, req *wrapperspb.StringValue) (
	*protos.ListResponse, error) {
	cfgName := req.Value
	if cfgName == "" {
		s.Logger.Trace("looking up all available configurations on server")
		return &protos.ListResponse{Ids: s.Store.GetAllConfigs()}, nil
	}
	s.Logger.Trace("looking up all version for configuration %s", cfgName)
	return &protos.ListResponse{Ids: s.Store.GetAllVersions(cfgName)}, nil
}

func (s *grpcSubscriber) GetConfiguration(ctx context.Context, configId *wrapperspb.StringValue) (
	*protos.Configuration, error) {
	cfgId := configId.Value
	s.Logger.Trace("retrieving Configuration %s", cfgId)
	cfg, found := s.Store.GetConfig(cfgId)
	if !found {
		return nil, status.Errorf(codes.NotFound, "configuration %s not found", cfgId)
	}
	return cfg, nil
}

func (s *grpcSubscriber) PutFiniteStateMachine(ctx context.Context,
	request *protos.PutFsmRequest) (*protos.PutResponse, error) {
	fsm := request.Fsm
	// First check that the configuration for the FSM is valid
	cfg, ok := s.Store.GetConfig(fsm.ConfigId)
	if !ok {
		return nil, status.Error(codes.FailedPrecondition, storage.NotFoundError(
			fsm.ConfigId).Error())
	}
	if deadline, ok := ctx.Deadline(); ok {
		if deadline.Before(time.Now()) {
			return nil, ctx.Err()
		}
	}
	var id = request.Id
	if id == "" {
		id = uuid.NewString()
	}
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
	// we cannot interrupt here, even if deadline is passed, as it would leave the
	// store in an inconsistent state.
	if err := s.Store.UpdateState(cfg.Name, id, "", fsm.State); err != nil {
		s.Logger.Error("could not store FSM in state set [%s]: %v", fsm.State, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &protos.PutResponse{Id: id, EntityResponse: &protos.PutResponse_Fsm{Fsm: fsm}}, nil
}

func (s *grpcSubscriber) GetFiniteStateMachine(ctx context.Context, in *protos.GetFsmRequest) (
	*protos.FiniteStateMachine, error) {
	cfg := in.GetConfig()
	if cfg == "" {
		return nil, status.Error(codes.InvalidArgument, "configuration name must always be provided when looking up statemachine")
	}
	fsmId := in.GetId()
	if fsmId == "" {
		return nil, status.Error(codes.InvalidArgument, "ID must always be provided when looking up statemachine")
	}
	s.Logger.Debug("looking up FSM [%s] (Configuration: %s)", fsmId, cfg)
	fsm, ok := s.Store.GetStateMachine(fsmId, cfg)
	if !ok {
		return nil, status.Error(codes.NotFound, storage.NotFoundError(fsmId).Error())
	}
	return fsm, nil
}

func (s *grpcSubscriber) GetAllInState(ctx context.Context, in *protos.GetFsmRequest) (
	*protos.ListResponse, error) {
	cfg := in.GetConfig()
	if cfg == "" {
		return nil, status.Errorf(codes.InvalidArgument, "configuration must always be specified")
	}
	state := in.GetState()
	if state == "" {
		// TODO: implement table scanning
		return nil, status.Errorf(codes.Unimplemented, "missing state, table scan not implemented")
	}
	ids := s.Store.GetAllInState(cfg, state)
	return &protos.ListResponse{Ids: ids}, nil
}

func (s *grpcSubscriber) GetEventOutcome(ctx context.Context, in *protos.EventRequest) (
	*protos.EventResponse, error) {
	evtId := in.GetId()
	cfg := in.GetConfig()
	s.Logger.Debug("looking up EventOutcome %s (%s)", evtId, cfg)
	outcome, ok := s.Store.GetOutcomeForEvent(evtId, cfg)
	if !ok {
		return nil, status.Error(codes.NotFound, fmt.Sprintf("outcome for event %s not found", evtId))
	}
	return &protos.EventResponse{
		EventId: evtId,
		Outcome: outcome,
	}, nil
}

func (s *grpcSubscriber) StreamAllInstate(in *protos.GetFsmRequest, stream StatemachineStream) error {
	response, err := s.GetAllInState(context.Background(), in)
	if err != nil {
		return err
	}
	cfgName := in.GetConfig()
	for _, id := range response.GetIds() {
		fsm, found := s.Store.GetStateMachine(id, cfgName)
		if !found {
			return storage.NotFoundError(id)
		}
		if err = stream.SendMsg(&protos.PutResponse{
			Id:             id,
			EntityResponse: &protos.PutResponse_Fsm{Fsm: fsm},
		}); err != nil {
			s.Logger.Error("could not stream response back: %s", err)
			return err
		}
	}
	return nil
}

func (s *grpcSubscriber) StreamAllConfigurations(in *wrapperspb.StringValue, stream ConfigurationStream) error {
	if in.GetValue() == "" {
		return status.Errorf(codes.InvalidArgument, "must specify the Configuration name")
	}
	response, err := s.GetAllConfigurations(context.Background(), in)
	if err != nil {
		return nil
	}
	for _, cfgId := range response.GetIds() {
		cfg, found := s.Store.GetConfig(cfgId)
		if !found {
			return storage.NotFoundError(cfgId)
		}
		if err = stream.SendMsg(cfg); err != nil {
			return err
		}
	}
	return nil
}

// NewGrpcServer creates a new gRPC server to handle incoming events and other API calls.
// The `Config` can be used to configure the backing store, a timeout and the logger.
func NewGrpcServer(cfg *Config) (*grpc.Server, error) {
	var creds credentials.TransportCredentials
	if !cfg.TlsEnabled {
		logging.RootLog.Warn("TLS is disabled for gRPC server, using insecure credentials")
		creds = insecure.NewCredentials()
	} else {
		logging.RootLog.Debug("TLS is enabled for gRPC server")
		serverTLSConfig, err := SetupTLSConfig(cfg)
		if err != nil {
			logging.RootLog.Error("could not setup TLS config: %q", err)
			return nil, err
		}
		creds = credentials.NewTLS(serverTLSConfig)
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = DefaultTimeout
	}
	server := grpc.NewServer(grpc.Creds(creds))
	protos.RegisterStatemachineServiceServer(server, &grpcSubscriber{Config: cfg})
	return server, nil
}

// TODO: move to its own grpc_tls.go file in this package.
func SetupTLSConfig(cfg *Config) (*tls.Config, error) {
	var err error
	var tlsConfig = &tls.Config{
		Certificates: make([]tls.Certificate, 1),
	}
	if cfg.TlsCerts == "" {
		certsDir := os.Getenv(config.TlsConfigDirEnv)
		if certsDir != "" {
			cfg.TlsCerts = certsDir
		} else {
			cfg.TlsCerts = config.DefaultConfigDir
		}
	}
	certFile := filepath.Join(cfg.TlsCerts, config.ServerCertFile)
	keyFile := filepath.Join(cfg.TlsCerts, config.ServerKeyFile)
	cfg.Logger.Info("setting up TLS: Server Certificate: %s, Key: %s", certFile, keyFile)
	tlsConfig.Certificates[0], err = tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		cfg.Logger.Error("cannot load certs: %s", err)
		return nil, err
	}
	cfg.Logger.Info("Adding CA Cert: %s/%s", cfg.TlsCerts, config.CAFile)
	ca, err := ParseCAFile(filepath.Join(cfg.TlsCerts, config.CAFile))
	if err != nil {
		return nil, err
	}
	if cfg.TlsMutual {
		tlsConfig.ClientCAs = ca
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	} else {
		tlsConfig.ClientAuth = tls.NoClientCert
	}
	tlsConfig.ServerName = cfg.ServerAddress
	return tlsConfig, nil
}

func ParseCAFile(caFile string) (*x509.CertPool, error) {
	_, err := os.Stat(caFile)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(caFile)
	if err != nil {
		return nil, err
	}
	ca := x509.NewCertPool()
	ok := ca.AppendCertsFromPEM(b)
	if !ok {
		return nil, fmt.Errorf("failed to parse root certificate: %q", caFile)
	}
	return ca, nil
}
