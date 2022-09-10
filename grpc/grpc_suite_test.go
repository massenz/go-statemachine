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
	"crypto/x509"
	"fmt"
	"github.com/massenz/go-statemachine/internal/config"
	internals "github.com/massenz/go-statemachine/internal/testing"
	protos "github.com/massenz/statemachine-proto/golang/api"
	g "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestGrpc(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "gRPC Server")
}

var container *internals.Container
var _ = BeforeSuite(func() {
	var err error
	container, err = internals.NewRedisContainer(context.Background())
	Expect(err).ToNot(HaveOccurred())
	// Note the timeout here is in seconds (and it's not a time.Duration either)
}, 5.0)

var _ = AfterSuite(func() {
	if container != nil {
		Expect(container.Terminate(context.Background())).To(Succeed())
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
		ca := filepath.Join("../certs", config.CAFile)
		_, err := os.Stat(ca)
		Expect(err).ToNot(HaveOccurred())

		clientTlsConfig, _ := setupClientTLSConfig(ca)
		clientTlsConfig.ServerName = address
		creds = credentials.NewTLS(clientTlsConfig)
	} else {
		creds = insecure.NewCredentials()
	}
	cc, _ := g.Dial(address, g.WithTransportCredentials(creds))
	client := protos.NewStatemachineServiceClient(cc)
	return client
}

func setupClientTLSConfig(caFile string) (*tls.Config, error) {
	var err error
	var tlsConfig = &tls.Config{
		Certificates: make([]tls.Certificate, 1),
	}
	_, err = os.Stat(caFile)
	if err != nil {
		b, err := os.ReadFile(caFile)
		if err != nil {
			return nil, err
		}
		ca := x509.NewCertPool()
		ok := ca.AppendCertsFromPEM(b)
		if !ok {
			return nil, fmt.Errorf("failed to parse root certificate: %q", caFile)
		}
		tlsConfig.RootCAs = ca
		tlsConfig.ClientAuth = tls.NoClientCert
	}
	return tlsConfig, nil
}
