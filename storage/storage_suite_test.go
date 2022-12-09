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
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"testing"
	"time"
)

func TestStorage(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Storage Suite")
}

type RedisContainer struct {
	testcontainers.Container
	Address string
}

func SetupRedis(ctx context.Context) (*RedisContainer, error) {
	req := testcontainers.ContainerRequest{
		Image:        "redis:6",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForLog("* Ready to accept connections"),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, err
	}

	mappedPort, err := container.MappedPort(ctx, "6379")
	if err != nil {
		return nil, err
	}

	hostIP, err := container.Host(ctx)
	if err != nil {
		return nil, err
	}

	address := fmt.Sprintf("%s:%s", hostIP, mappedPort.Port())
	return &RedisContainer{Container: container, Address: address}, nil
}

var container *RedisContainer
var _ = BeforeSuite(func() {
	var err error
	container, err = SetupRedis(context.Background())
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
