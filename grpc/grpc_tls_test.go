/*
 * Copyright (c) 2023 AlertAvert.com.  All rights reserved.
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
	"github.com/massenz/go-statemachine/storage"
	slf4go "github.com/massenz/slf4go/logging"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"math/rand"
	"net"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/massenz/go-statemachine/grpc"
	protos "github.com/massenz/statemachine-proto/golang/api"
)

var _ = Describe("gRPC Server with TLS", func() {
	When("processing events", func() {
		var testCh chan protos.EventRequest
		var listener net.Listener
		var client protos.StatemachineServiceClient
		var done func()
		var addr string

		BeforeEach(func() {
			var err error
			addr = fmt.Sprintf("localhost:%d", (rand.Int()%25535)+10000)
			testCh = make(chan protos.EventRequest, 5)
			listener, err = net.Listen("tcp", addr)
			Ω(err).ShouldNot(HaveOccurred())

			// TODO: use GinkgoWriter for logs
			l := slf4go.NewLog("grpc-TLS-test")
			l.Level = slf4go.NONE
			server, err := grpc.NewGrpcServer(&grpc.Config{
				EventsChannel: testCh,
				Logger:        l,
				ServerAddress: addr,
				Store:         storage.NewRedisStoreWithDefaults(redisContainer.Address),
				TlsEnabled:    true,
				TlsCerts:      "../certs",
				// TODO: add mTLS tests
				TlsMutual: false,
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
		AfterEach(func() {
			done()
		})
		It("should connect a client using TLS", func() {
			client = NewClient(addr, true)
			ctx, cancel := context.WithTimeout(bkgnd, 300*time.Millisecond)
			defer cancel()
			_, err := client.GetAllConfigurations(ctx, &wrapperspb.StringValue{Value: "test.orders"})
			Ω(err).ToNot(HaveOccurred())
		})
		It("should refuse non TLS connections", func() {
			ctx, cancel := context.WithTimeout(bkgnd, 300*time.Millisecond)
			defer cancel()
			client = NewClient(addr, false)
			_, err := client.GetAllConfigurations(ctx, &wrapperspb.StringValue{Value: "test.orders"})
			AssertStatusCode(codes.Unavailable, err)
		})
	})
})
