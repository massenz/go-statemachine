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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	internals "github.com/massenz/go-statemachine/internals/testing"
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
