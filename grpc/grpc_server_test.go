/*
 * Copyright (c) 2022 AlertAvert.com.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Author: Marco Massenzio (marco@alertavert.com)
 */

package grpc_test

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/massenz/slf4go/logging"
	g "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/wrapperspb"

	. "github.com/JiaYongfei/respect/gomega"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/massenz/go-statemachine/api"
	"github.com/massenz/go-statemachine/grpc"
	"github.com/massenz/go-statemachine/storage"
	"github.com/massenz/statemachine-proto/golang/api"
)

var _ = Describe("the gRPC Server", func() {

	When("processing events", func() {
		var testCh chan api.EventRequest
		var listener net.Listener
		var client api.StatemachineServiceClient
		var done func()

		BeforeEach(func() {
			var err error
			testCh = make(chan api.EventRequest, 5)
			listener, err = net.Listen("tcp", ":0")
			Ω(err).ShouldNot(HaveOccurred())

			cc, err := g.Dial(listener.Addr().String(),
				g.WithTransportCredentials(insecure.NewCredentials()))
			Ω(err).ShouldNot(HaveOccurred())

			client = api.NewStatemachineServiceClient(cc)
			// TODO: use GinkgoWriter for logs
			l := logging.NewLog("grpc-server-test")
			l.Level = logging.NONE
			// Note the `Config` here has no store configured, because we are
			// only testing events in this Context, and those are never stored
			// in Redis by the gRPC server (other parts of the system do).
			server, err := grpc.NewGrpcServer(&grpc.Config{
				EventsChannel: testCh,
				Logger:        l,
			})
			Ω(err).ToNot(HaveOccurred())
			Ω(server).ToNot(BeNil())

			go func() {
				Ω(server.Serve(listener)).Should(Succeed())
			}()
			done = func() {
				server.Stop()
			}
		})
		It("should succeed for well-formed events", func() {
			response, err := client.ProcessEvent(context.Background(), &api.EventRequest{
				Event: &api.Event{
					EventId: "1",
					Transition: &api.Transition{
						Event: "test-vt",
					},
					Originator: "test",
				},
				Dest: "2",
			})
			Ω(err).ToNot(HaveOccurred())
			Ω(response).ToNot(BeNil())
			Ω(response.EventId).To(Equal("1"))
			done()
			select {
			case evt := <-testCh:
				Ω(evt.Event.EventId).To(Equal("1"))
				Ω(evt.Event.Transition.Event).To(Equal("test-vt"))
				Ω(evt.Event.Originator).To(Equal("test"))
				Ω(evt.Dest).To(Equal("2"))
			case <-time.After(10 * time.Millisecond):
				Fail("Timed out")

			}
		})
		It("should create an ID for events without", func() {
			response, err := client.ProcessEvent(context.Background(), &api.EventRequest{
				Event: &api.Event{
					Transition: &api.Transition{
						Event: "test-vt",
					},
					Originator: "test",
				},
				Dest: "123456",
			})
			Ω(err).ToNot(HaveOccurred())
			Ω(response.EventId).ToNot(BeNil())
			generatedId := response.EventId
			done()
			select {
			case evt := <-testCh:
				Ω(evt.Event.EventId).Should(Equal(generatedId))
				Ω(evt.Event.Transition.Event).To(Equal("test-vt"))
			case <-time.After(10 * time.Millisecond):
				Fail("Timed out")
			}
		})
		It("should fail for missing destination", func() {
			_, err := client.ProcessEvent(context.Background(), &api.EventRequest{
				Event: &api.Event{
					Transition: &api.Transition{
						Event: "test-vt",
					},
					Originator: "test",
				},
			})
			AssertStatusCode(codes.FailedPrecondition, err)
			done()
			select {
			case evt := <-testCh:
				Fail(fmt.Sprintf("Unexpected event: %s", evt.String()))
			case <-time.After(10 * time.Millisecond):
				Succeed()
			}
		})
		It("should fail for missing event", func() {
			_, err := client.ProcessEvent(context.Background(), &api.EventRequest{
				Event: &api.Event{
					Transition: &api.Transition{
						Event: "",
					},
					Originator: "test",
				},
				Dest: "9876",
			})
			AssertStatusCode(codes.FailedPrecondition, err)
			done()
			select {
			case evt := <-testCh:
				Fail(fmt.Sprintf("UnΩed event: %s", evt.String()))
			case <-time.After(10 * time.Millisecond):
				Succeed()
			}
		})
	})

	When("using Redis as the backing store", func() {
		var (
			listener net.Listener
			client   api.StatemachineServiceClient
			cfg      *api.Configuration
			fsm      *api.FiniteStateMachine
			done     func()
			store    storage.StoreManager
		)

		// Server setup
		BeforeEach(func() {
			store = storage.NewRedisStoreWithDefaults(container.Address)
			store.SetLogLevel(logging.NONE)
			listener, _ = net.Listen("tcp", ":0")
			cc, _ := g.Dial(listener.Addr().String(),
				g.WithTransportCredentials(insecure.NewCredentials()))
			client = api.NewStatemachineServiceClient(cc)
			// Use this to log errors when diagnosing test failures; then set to NONE once done.
			l := logging.NewLog("grpc-server-test")
			l.Level = logging.NONE
			server, _ := grpc.NewGrpcServer(&grpc.Config{
				Store:  store,
				Logger: l,
			})

			go func() {
				Ω(server.Serve(listener)).Should(Succeed())
			}()
			done = func() {
				server.Stop()
			}
		})
		// Server shutdown & Clean up the DB
		AfterEach(func() {
			done()
			rdb := redis.NewClient(&redis.Options{
				Addr: container.Address,
				DB:   storage.DefaultRedisDb,
			})
			rdb.FlushDB(context.Background())

		})
		Context("handling Configuration API requests", func() {
			// Test data setup
			BeforeEach(func() {
				cfg = &api.Configuration{
					Name:    "test-conf",
					Version: "v1",
					States:  []string{"start", "stop"},
					Transitions: []*api.Transition{
						{From: "start", To: "stop", Event: "shutdown"},
					},
					StartingState: "start",
				}
			})
			It("should store valid configurations", func() {
				_, ok := store.GetConfig(GetVersionId(cfg))
				Ω(ok).To(BeFalse())
				response, err := client.PutConfiguration(context.Background(), cfg)
				Ω(err).ToNot(HaveOccurred())
				Ω(response).ToNot(BeNil())
				Ω(response.Id).To(Equal(GetVersionId(cfg)))
				found, ok := store.GetConfig(response.Id)
				Ω(ok).Should(BeTrue())
				Ω(found).Should(Respect(cfg))
			})
			It("should fail for invalid configuration", func() {
				invalid := &api.Configuration{
					Name:          "invalid",
					Version:       "v1",
					States:        []string{},
					Transitions:   nil,
					StartingState: "",
				}
				_, err := client.PutConfiguration(context.Background(), invalid)
				AssertStatusCode(codes.InvalidArgument, err)
			})
			It("should retrieve a valid configuration", func() {
				Ω(store.PutConfig(cfg)).To(Succeed())
				response, err := client.GetConfiguration(context.Background(),
					&wrapperspb.StringValue{Value: GetVersionId(cfg)})
				Ω(err).ToNot(HaveOccurred())
				Ω(response).ToNot(BeNil())
				Ω(response).Should(Respect(cfg))
			})
			It("should return an empty configuration for an invalid ID", func() {
				_, err := client.GetConfiguration(context.Background(), &wrapperspb.StringValue{Value: "fake"})
				AssertStatusCode(codes.NotFound, err)
			})
		})
		Context("handling Statemachine API requests", func() {
			// Test data setup
			BeforeEach(func() {
				cfg = &api.Configuration{
					Name:    "test-conf",
					Version: "v1",
					States:  []string{"start", "stop"},
					Transitions: []*api.Transition{
						{From: "start", To: "stop", Event: "shutdown"},
					},
					StartingState: "start",
				}
				fsm = &api.FiniteStateMachine{ConfigId: GetVersionId(cfg)}
			})
			It("should store a valid FSM", func() {
				Ω(store.PutConfig(cfg)).To(Succeed())
				resp, err := client.PutFiniteStateMachine(context.Background(),
					&api.PutFsmRequest{Id: "123456", Fsm: fsm})
				Ω(err).ToNot(HaveOccurred())
				Ω(resp).ToNot(BeNil())
				Ω(resp.Id).To(Equal("123456"))
				Ω(resp.Fsm).Should(Respect(fsm))
			})
			It("should fail with an invalid Config ID", func() {
				invalid := &api.FiniteStateMachine{ConfigId: "fake"}
				_, err := client.PutFiniteStateMachine(context.Background(),
					&api.PutFsmRequest{Fsm: invalid})
				AssertStatusCode(codes.FailedPrecondition, err)
			})
			It("can retrieve a stored FSM", func() {
				id := "123456"
				Ω(store.PutConfig(cfg))
				Ω(store.PutStateMachine(id, fsm)).Should(Succeed())
				Ω(client.GetFiniteStateMachine(context.Background(),
					&wrapperspb.StringValue{
						Value: strings.Join([]string{cfg.Name, id}, storage.KeyPrefixIDSeparator),
					})).Should(Respect(fsm))
			})
			It("will return an Invalid error for a malformed ID", func() {
				_, err := client.GetFiniteStateMachine(context.Background(), &wrapperspb.StringValue{Value: "fake"})
				AssertStatusCode(codes.InvalidArgument, err)
			})
			It("will return a NotFound error for a missing ID", func() {
				_, err := client.GetFiniteStateMachine(context.Background(),
					&wrapperspb.StringValue{Value: "cfg#fake"})
				AssertStatusCode(codes.NotFound, err)
			})
			It("will find all configurations", func() {
				names := []string{"orders", "devices", "users"}
				for _, name := range names {
					cfg = &api.Configuration{
						Name:    name,
						Version: "v1",
						States:  []string{"start", "stop"},
						Transitions: []*api.Transition{
							{From: "start", To: "stop", Event: "shutdown"},
						},
						StartingState: "start",
					}
					Ω(store.PutConfig(cfg)).Should(Succeed())
				}
				found, err := client.GetAllConfigurations(context.Background(), &wrapperspb.StringValue{})
				Ω(err).Should(Succeed())
				Ω(len(found.Ids)).To(Equal(3))
				for _, value := range found.Ids {
					Ω(names).To(ContainElement(value))
				}
			})
		})
	})
})
