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
	"github.com/go-redis/redis/v8"
	"github.com/massenz/slf4go/logging"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	g "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"io"
	"net"

	. "github.com/massenz/go-statemachine/api"
	"github.com/massenz/go-statemachine/grpc"
	"github.com/massenz/go-statemachine/storage"
	"github.com/massenz/statemachine-proto/golang/api"
)

var _ = Describe("gRPC Server Streams", func() {
	When("using Redis as the backing store", func() {
		var (
			listener net.Listener
			client   api.StatemachineServiceClient
			cfg      *api.Configuration
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
		Context("streaming Configurations", func() {
			var versions []string
			var name = "test-conf"
			// Test data setup
			BeforeEach(func() {
				versions = []string{"v1", "v2", "v3"}
				cfg = &api.Configuration{
					Name:   name,
					States: []string{"start", "stop"},
					Transitions: []*api.Transition{
						{From: "start", To: "stop", Event: "shutdown"},
					},
					StartingState: "start",
				}
				for _, v := range versions {
					cfg.Version = v
					Ω(store.PutConfig(cfg)).ToNot(HaveOccurred())
				}
			})
			It("should find all configurations", func() {
				stream, err := client.StreamAllConfigurations(bkgnd,
					&wrapperspb.StringValue{Value: name})
				Ω(err).ShouldNot(HaveOccurred())
				count := 0
				for {
					item, err := stream.Recv()
					if err == io.EOF {
						Ω(count).Should(Equal(len(versions)))
						break
					}
					count++
					Ω(err).ShouldNot(HaveOccurred())
					Ω(item).ShouldNot(BeNil())
					Ω(item.Name).Should(Equal(name))
					Ω(versions).Should(ContainElement(item.Version))
				}
			})
			It("should fail for empty name", func() {
				data, err := client.StreamAllConfigurations(bkgnd, &wrapperspb.StringValue{Value: ""})
				Ω(err).ShouldNot(HaveOccurred())
				_, err = data.Recv()
				Ω(err).Should(HaveOccurred())
				AssertStatusCode(codes.InvalidArgument, err)
			})
			It("should retrieve an empty stream for valid but non-existent configuration", func() {
				response, err := client.StreamAllConfigurations(bkgnd,
					&wrapperspb.StringValue{Value: "fake"})
				Ω(err).ShouldNot(HaveOccurred())
				Ω(response).ShouldNot(BeNil())
				_, err = response.Recv()
				Ω(err).Should(Equal(io.EOF))
			})
		})
		Context("streaming Statemachines", func() {
			var ids = []string{"1", "2", "3"}
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
				Ω(store.PutConfig(cfg)).ShouldNot(HaveOccurred())
				for _, id := range ids {
					Ω(store.PutStateMachine(id, &api.FiniteStateMachine{
						ConfigId: GetVersionId(cfg),
						State:    "start",
					})).ShouldNot(HaveOccurred())
					Ω(store.UpdateState(cfg.Name, id, "", "start")).
						ShouldNot(HaveOccurred())
				}
			})
			It("should find all FSM", func() {
				resp, err := client.StreamAllInstate(bkgnd,
					&api.GetAllFsmRequest{
						Config: &wrapperspb.StringValue{Value: cfg.Name},
						State:  &wrapperspb.StringValue{Value: "start"},
					})
				Ω(err).ShouldNot(HaveOccurred())
				Ω(resp).ShouldNot(BeNil())
				count := 0
				for {
					item, err := resp.Recv()
					if err == io.EOF {
						Ω(count).Should(Equal(len(ids)))
						break
					}
					count++
					Ω(err).ShouldNot(HaveOccurred())
					Ω(ids).Should(ContainElement(item.Id))
					fsm := item.GetFsm()
					Ω(fsm).ShouldNot(BeNil())
					Ω(fsm.State).Should(Equal("start"))
					Ω(fsm.ConfigId).Should(Equal(GetVersionId(cfg)))
				}
			})
		})
	})
})
