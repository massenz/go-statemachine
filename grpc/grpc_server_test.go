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
	. "github.com/JiaYongfei/respect/gomega"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
	"strings"

	"context"
	"fmt"
	"github.com/massenz/slf4go/logging"
	g "google.golang.org/grpc"
	"google.golang.org/grpc/status"
	"net"
	"time"

	. "github.com/massenz/go-statemachine/api"
	"github.com/massenz/go-statemachine/grpc"
	"github.com/massenz/go-statemachine/storage"
	"github.com/massenz/statemachine-proto/golang/api"
)

var _ = Describe("GrpcServer", func() {

	Context("when processing events", func() {
		var testCh chan api.EventRequest
		var listener net.Listener
		var client api.StatemachineServiceClient
		var done func()

		BeforeEach(func() {
			var err error
			testCh = make(chan api.EventRequest, 5)
			listener, err = net.Listen("tcp", ":0")
			Ω(err).ShouldNot(HaveOccurred())

			cc, err := g.Dial(listener.Addr().String(), g.WithInsecure())
			Ω(err).ShouldNot(HaveOccurred())

			client = api.NewStatemachineServiceClient(cc)
			l := logging.NewLog("grpc-server-test")
			l.Level = logging.NONE
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
			assertStatusCode(codes.FailedPrecondition, err)
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
			assertStatusCode(codes.FailedPrecondition, err)
			done()
			select {
			case evt := <-testCh:
				Fail(fmt.Sprintf("UnΩed event: %s", evt.String()))
			case <-time.After(10 * time.Millisecond):
				Succeed()
			}
		})
	})

	Context("when retrieving data from the store", func() {
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
			store = storage.NewInMemoryStore()
			store.SetLogLevel(logging.NONE)

			listener, _ = net.Listen("tcp", ":0")
			cc, _ := g.Dial(listener.Addr().String(), g.WithInsecure())
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
		// Server shutdown
		AfterEach(func() {
			done()
		})
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
			assertStatusCode(codes.InvalidArgument, err)
		})
		It("should retrieve a valid configuration", func() {
			Ω(store.PutConfig(cfg)).To(Succeed())
			response, err := client.GetConfiguration(context.Background(),
				&api.GetRequest{Id: GetVersionId(cfg)})
			Ω(err).ToNot(HaveOccurred())
			Ω(response).ToNot(BeNil())
			Ω(response).Should(Respect(cfg))
		})
		It("should return an empty configuration for an invalid ID", func() {
			_, err := client.GetConfiguration(context.Background(), &api.GetRequest{Id: "fake"})
			assertStatusCode(codes.NotFound, err)
		})
		It("should store a valid FSM", func() {
			Ω(store.PutConfig(cfg)).To(Succeed())
			resp, err := client.PutFiniteStateMachine(context.Background(), fsm)
			Ω(err).ToNot(HaveOccurred())
			Ω(resp).ToNot(BeNil())
			Ω(resp.Id).ToNot(BeNil())
			Ω(resp.Fsm).Should(Respect(fsm))
		})
		It("should fail with an invalid Config ID", func() {
			invalid := &api.FiniteStateMachine{ConfigId: "fake"}
			_, err := client.PutFiniteStateMachine(context.Background(), invalid)
			assertStatusCode(codes.FailedPrecondition, err)
		})
		It("can retrieve a stored FSM", func() {
			id := "123456"
			Ω(store.PutConfig(cfg))
			Ω(store.PutStateMachine(id, fsm)).Should(Succeed())
			Ω(client.GetFiniteStateMachine(context.Background(),
				&api.GetRequest{
					Id: strings.Join([]string{cfg.Name, id}, storage.KeyPrefixIDSeparator),
				})).Should(Respect(fsm))
		})
		It("will return an Invalid error for an invalid ID", func() {
			_, err := client.GetFiniteStateMachine(context.Background(), &api.GetRequest{Id: "fake"})
			assertStatusCode(codes.InvalidArgument, err)
		})
		It("will return a NotFound error for a missing ID", func() {
			_, err := client.GetFiniteStateMachine(context.Background(),
				&api.GetRequest{Id: "cfg#fake"})
			assertStatusCode(codes.NotFound, err)
		})
	})
})

func assertStatusCode(code codes.Code, err error) {
	Ω(err).To(HaveOccurred())
	s, ok := status.FromError(err)
	Ω(ok).To(BeTrue())
	Ω(s.Code()).To(Equal(code))
}
