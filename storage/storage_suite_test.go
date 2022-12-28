/*
 * Copyright (c) 2022 AlertAvert.com.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Author: Marco Massenzio (marco@alertavert.com)
 */

package storage_test

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	internals "github.com/massenz/go-statemachine/internals/testing"
)

func TestStorage(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Storage Suite")
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
		timeout, _ := time.ParseDuration("2s")
		err := container.Stop(context.Background(), &timeout)
		Expect(err).ToNot(HaveOccurred())
	}
}, 2.0)
