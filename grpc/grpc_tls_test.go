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
	slf4go "github.com/massenz/slf4go/logging"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"net"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/massenz/go-statemachine/grpc"
	protos "github.com/massenz/statemachine-proto/golang/api"
)

var _ = Describe("the gRPC Server", func() {
	When("processing events", func() {
		var testCh chan protos.EventRequest
		var listener net.Listener
		var client protos.StatemachineServiceClient

		BeforeEach(func() {
			var err error
			testCh = make(chan protos.EventRequest, 5)
			listener, err = net.Listen("tcp", "localhost:5764")
			Ω(err).ShouldNot(HaveOccurred())

			// TODO: use GinkgoWriter for logs
			l := slf4go.NewLog("grpc-TLS-test")
			l.Level = slf4go.DEBUG

			client = NewClient(listener.Addr().String(), true)

			server, err := grpc.NewGrpcServer(&grpc.Config{
				EventsChannel: testCh,
				Logger:        l,
				ServerAddress: listener.Addr().String(),
				TlsEnabled:    true,
				TlsCerts:      "../certs",
			})
			Ω(err).ToNot(HaveOccurred())
			Ω(server).ToNot(BeNil())

		})
		It("should connect using TLS", func() {
			_, err := client.GetAllConfigurations(bkgnd, &wrapperspb.StringValue{Value: "test"})
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
