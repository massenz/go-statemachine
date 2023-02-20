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
	slf4go "github.com/massenz/slf4go/logging"
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
	protos "github.com/massenz/statemachine-proto/golang/api"
)

var bkgnd = context.Background()
var _ = Describe("the gRPC Server", func() {
	When("processing events", func() {
		var testCh chan protos.EventRequest
		var listener net.Listener
		var client protos.StatemachineServiceClient
		var done func()

		BeforeEach(func() {
			var err error
			testCh = make(chan protos.EventRequest, 5)
			listener, err = net.Listen("tcp", "localhost:5763")
			Ω(err).ShouldNot(HaveOccurred())

			client = NewClient(listener.Addr().String(), false)
			// TODO: use GinkgoWriter for logs
			l := slf4go.NewLog("grpc-server-test")
			l.Level = slf4go.DEBUG
			// Note the `Config` here has no store configured, because we are
			// only testing events in this Context, and those are never stored
			// in Redis by the gRPC server (other parts of the system do).
			server, err := grpc.NewGrpcServer(&grpc.Config{
				EventsChannel: testCh,
				Logger:        l,
				ServerAddress: listener.Addr().String(),
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
			response, err := client.SendEvent(bkgnd, &protos.EventRequest{
				Event: &protos.Event{
					EventId: "1",
					Transition: &protos.Transition{
						Event: "test-vt",
					},
					Originator: "test",
				},
				Config: "test-cfg",
				Id:     "2",
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
				Ω(evt.Id).To(Equal("2"))
			case <-time.After(10 * time.Millisecond):
				Fail("Timed out")

			}
		})
		It("should create an ID for events without", func() {
			response, err := client.SendEvent(bkgnd, &protos.EventRequest{
				Event: &protos.Event{
					Transition: &protos.Transition{
						Event: "test-vt",
					},
					Originator: "test",
				},
				Config: "test-cfg",
				Id:     "123456",
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
			_, err := client.SendEvent(bkgnd, &protos.EventRequest{
				Event: &protos.Event{
					Transition: &protos.Transition{
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
			_, err := client.SendEvent(bkgnd, &protos.EventRequest{
				Event: &protos.Event{
					Transition: &protos.Transition{
						Event: "",
					},
					Originator: "test",
				},
				Config: "test",
				Id:     "9876",
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
			client   protos.StatemachineServiceClient
			cfg      *protos.Configuration
			fsm      *protos.FiniteStateMachine
			done     func()
			store    storage.StoreManager
		)

		// Server setup
		BeforeEach(func() {
			store = storage.NewRedisStoreWithDefaults(redisContainer.Address)
			store.SetLogLevel(slf4go.NONE)
			listener, _ = net.Listen("tcp", ":0")
			cc, _ := g.Dial(listener.Addr().String(),
				g.WithTransportCredentials(insecure.NewCredentials()))
			client = protos.NewStatemachineServiceClient(cc)
			// Use this to log errors when diagnosing test failures; then set to NONE once done.
			l := slf4go.NewLog("grpc-server-test")
			l.Level = slf4go.NONE
			server, _ := grpc.NewGrpcServer(&grpc.Config{
				Store:  store,
				Logger: l,
			})

			go func() {
				defer GinkgoRecover()
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
				Addr: redisContainer.Address,
				DB:   storage.DefaultRedisDb,
			})
			rdb.FlushDB(context.Background())
		})
		Context("handling Configuration API requests", func() {
			// Test data setup
			BeforeEach(func() {
				cfg = &protos.Configuration{
					Name:    "test-conf",
					Version: "v1",
					States:  []string{"start", "stop"},
					Transitions: []*protos.Transition{
						{From: "start", To: "stop", Event: "shutdown"},
					},
					StartingState: "start",
				}
			})
			It("should store valid configurations", func() {
				_, ok := store.GetConfig(GetVersionId(cfg))
				Ω(ok).To(BeFalse())
				response, err := client.PutConfiguration(bkgnd, cfg)
				Ω(err).ToNot(HaveOccurred())
				Ω(response).ToNot(BeNil())
				Ω(response.Id).To(Equal(GetVersionId(cfg)))
				found, ok := store.GetConfig(response.Id)
				Ω(ok).Should(BeTrue())
				Ω(found).Should(Respect(cfg))
			})
			It("should fail for invalid configuration", func() {
				invalid := &protos.Configuration{
					Name:          "invalid",
					Version:       "v1",
					States:        []string{},
					Transitions:   nil,
					StartingState: "",
				}
				_, err := client.PutConfiguration(bkgnd, invalid)
				AssertStatusCode(codes.InvalidArgument, err)
			})
			It("should retrieve a valid configuration", func() {
				Ω(store.PutConfig(cfg)).To(Succeed())
				response, err := client.GetConfiguration(bkgnd,
					&wrapperspb.StringValue{Value: GetVersionId(cfg)})
				Ω(err).ToNot(HaveOccurred())
				Ω(response).ToNot(BeNil())
				Ω(response).Should(Respect(cfg))
			})
			It("should return an empty configuration for an invalid ID", func() {
				_, err := client.GetConfiguration(bkgnd, &wrapperspb.StringValue{Value: "fake"})
				AssertStatusCode(codes.NotFound, err)
			})
			It("will find all configurations", func() {
				names := []string{"orders", "devices", "users"}
				for _, name := range names {
					cfg = &protos.Configuration{
						Name:    name,
						Version: "v1",
						States:  []string{"start", "stop"},
						Transitions: []*protos.Transition{
							{From: "start", To: "stop", Event: "shutdown"},
						},
						StartingState: "start",
					}
					Ω(store.PutConfig(cfg)).Should(Succeed())
				}
				found, err := client.GetAllConfigurations(bkgnd, &wrapperspb.StringValue{})
				Ω(err).Should(Succeed())
				Ω(len(found.Ids)).To(Equal(3))
				for _, value := range found.Ids {
					Ω(names).To(ContainElement(value))
				}
			})
			It("will find all version for a configuration", func() {
				name := "store.api"
				versions := []string{"v1alpha", "v1beta", "v1"}
				for _, v := range versions {
					cfg = &protos.Configuration{
						Name:    name,
						Version: v,
						States:  []string{"checkout", "close"},
						Transitions: []*protos.Transition{
							{From: "checkout", To: "close", Event: "payment"},
						},
						StartingState: "checkout",
					}
					Ω(store.PutConfig(cfg)).Should(Succeed())
				}
				found, err := client.GetAllConfigurations(bkgnd, &wrapperspb.StringValue{Value: name})
				Ω(err).Should(Succeed())
				Ω(len(found.Ids)).To(Equal(3))
				for _, value := range versions {
					Ω(found.Ids).To(ContainElement(
						strings.Join([]string{name, value}, storage.KeyPrefixComponentsSeparator)))
				}
			})
		})
		Context("handling Statemachine API requests", func() {
			// Test data setup
			BeforeEach(func() {
				cfg = &protos.Configuration{
					Name:    "test-conf",
					Version: "v1",
					States:  []string{"start", "stop"},
					Transitions: []*protos.Transition{
						{From: "start", To: "stop", Event: "shutdown"},
					},
					StartingState: "start",
				}
				fsm = &protos.FiniteStateMachine{ConfigId: GetVersionId(cfg)}
			})
			It("should store a valid FSM", func() {
				Ω(store.PutConfig(cfg)).To(Succeed())
				resp, err := client.PutFiniteStateMachine(bkgnd,
					&protos.PutFsmRequest{Id: "123456", Fsm: fsm})
				Ω(err).ToNot(HaveOccurred())
				Ω(resp).ToNot(BeNil())
				Ω(resp.Id).To(Equal("123456"))
				Ω(resp.GetFsm()).Should(Respect(fsm))
				// As we didn't specify a state when creating the FSM, the `StartingState`
				// was automatically configured.
				found := store.GetAllInState(cfg.Name, cfg.StartingState)
				Ω(len(found)).To(Equal(1))
				Ω(found[0]).To(Equal(resp.Id))
			})
			It("should fail with an invalid Config ID", func() {
				invalid := &protos.FiniteStateMachine{ConfigId: "fake"}
				_, err := client.PutFiniteStateMachine(bkgnd,
					&protos.PutFsmRequest{Fsm: invalid})
				AssertStatusCode(codes.FailedPrecondition, err)
			})
			It("can retrieve a stored FSM", func() {
				id := "123456"
				Ω(store.PutConfig(cfg))
				Ω(store.PutStateMachine(id, fsm)).Should(Succeed())
				Ω(client.GetFiniteStateMachine(bkgnd,
					&protos.GetFsmRequest{
						Config: cfg.Name,
						Query:  &protos.GetFsmRequest_Id{Id: id},
					})).Should(Respect(fsm))
			})
			It("will return an Invalid error for missing config or ID", func() {
				_, err := client.GetFiniteStateMachine(bkgnd,
					&protos.GetFsmRequest{
						Query: &protos.GetFsmRequest_Id{Id: "fake"},
					})
				AssertStatusCode(codes.InvalidArgument, err)
				_, err = client.GetFiniteStateMachine(bkgnd,
					&protos.GetFsmRequest{
						Config: cfg.Name,
					})
				AssertStatusCode(codes.InvalidArgument, err)
			})
			It("will return a NotFound error for a missing ID", func() {
				_, err := client.GetFiniteStateMachine(bkgnd,
					&protos.GetFsmRequest{
						Config: cfg.Name,
						Query:  &protos.GetFsmRequest_Id{Id: "12345"},
					})
				AssertStatusCode(codes.NotFound, err)
			})
			It("will find all FSMs by State", func() {
				for i := 1; i <= 5; i++ {
					id := fmt.Sprintf("fsm-%d", i)
					Ω(store.PutStateMachine(id,
						&protos.FiniteStateMachine{
							ConfigId: "test.m:v1",
							State:    "start",
						})).Should(Succeed())
					store.UpdateState("test.m", id, "", "start")
				}
				for i := 10; i < 13; i++ {
					id := fmt.Sprintf("fsm-%d", i)
					Ω(store.PutStateMachine(id,
						&protos.FiniteStateMachine{
							ConfigId: "test.m:v1",
							State:    "stop",
						})).Should(Succeed())
					store.UpdateState("test.m", id, "", "stop")

				}
				items, err := client.GetAllInState(bkgnd, &protos.GetFsmRequest{
					Config: "test.m",
					Query:  &protos.GetFsmRequest_State{State: "start"},
				})
				Ω(err).ShouldNot(HaveOccurred())
				Ω(len(items.GetIds())).Should(Equal(5))
				Ω(items.GetIds()).Should(ContainElements("fsm-3", "fsm-5"))
				items, err = client.GetAllInState(bkgnd, &protos.GetFsmRequest{
					Config: "test.m",
					Query:  &protos.GetFsmRequest_State{State: "stop"},
				})
				Ω(err).ShouldNot(HaveOccurred())
				Ω(len(items.GetIds())).Should(Equal(3))
				Ω(items.GetIds()).Should(ContainElements("fsm-10", "fsm-12"))
			})
		})
	})
})
