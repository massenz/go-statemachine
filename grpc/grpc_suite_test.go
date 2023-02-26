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
	"crypto/tls"
	"github.com/massenz/go-statemachine/grpc"
	internals "github.com/massenz/go-statemachine/internal/testing"
	slf4go "github.com/massenz/slf4go/logging"
	protos "github.com/massenz/statemachine-proto/golang/api"
	g "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestGrpc(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "gRPC Server")
}

var redisContainer *internals.Container
var _ = BeforeSuite(func() {
	var err error
	redisContainer, err = internals.NewRedisContainer(context.Background())
	Expect(err).ToNot(HaveOccurred())
	// Muting the RootLog to prevent annoying warning re TLS
	slf4go.RootLog.Level = slf4go.NONE
	// Note the timeout here is in seconds (and it's not a time.Duration either)
}, 5.0)

var _ = AfterSuite(func() {
	if redisContainer != nil {
		Expect(redisContainer.Terminate(context.Background())).To(Succeed())
	}
}, 2.0)

// TODO: should be an Omega Matcher
func AssertStatusCode(code codes.Code, err error) {
	Ω(err).To(HaveOccurred())
	s, ok := status.FromError(err)
	Ω(ok).To(BeTrue())
	Ω(s.Code()).To(Equal(code))
}

func NewClient(address string, isTls bool) protos.StatemachineServiceClient {
	var creds credentials.TransportCredentials
	if isTls {
		clientTlsConfig := &tls.Config{}
		ca, err := grpc.ParseCAFile("../certs/ca.pem")
		Expect(err).ToNot(HaveOccurred())
		clientTlsConfig.RootCAs = ca

		// NOTE: need to remove the :port from the address, or Cert validation will fail.
		addr := strings.Split(address, ":")
		Expect(len(addr)).Should(BeNumerically(">=", 1), addr)
		clientTlsConfig.ServerName = addr[0]

		creds = credentials.NewTLS(clientTlsConfig)
	} else {
		creds = insecure.NewCredentials()
	}
	cc, _ := g.Dial(address, g.WithTransportCredentials(creds))
	client := protos.NewStatemachineServiceClient(cc)
	return client
}
