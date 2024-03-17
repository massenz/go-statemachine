/*
 * Copyright (c) 2022 AlertAvert.com.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Author: Marco Massenzio (marco@alertavert.com)
 */

package testing

import (
	"context"
	"fmt"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	localstackImage    = "localstack/localstack:3.2"
	localstackEdgePort = "4566"
	redisImage         = "redis:6"
	redisPort          = "6379/tcp"
	Region             = "us-west-2"
)

// Container is an internal wrapper around the `testcontainers.Container` carrying also
// the `Address` (which could be a URI) to which the Server can be reached at.
type Container struct {
	testcontainers.Container
	Address string
}

// NewLocalstackContainer creates a new connection to the `LocalStack` `testcontainers`
func NewLocalstackContainer(ctx context.Context) (*Container, error) {
	req := testcontainers.ContainerRequest{
		Image:        localstackImage,
		ExposedPorts: []string{localstackEdgePort},
		WaitingFor:   wait.ForLog("Ready."),
		Env: map[string]string{
			"AWS_REGION": Region,
			"EDGE_PORT":  localstackEdgePort,
			"SERVICES":   "sqs",
		},
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, err
	}

	ip, err := container.Host(ctx)
	if err != nil {
		return nil, err
	}

	mappedPort, err := container.MappedPort(ctx, localstackEdgePort)
	if err != nil {
		return nil, err
	}

	uri := fmt.Sprintf("http://%s:%s", ip, mappedPort.Port())
	return &Container{Container: container, Address: uri}, nil
}

func NewRedisContainer(ctx context.Context) (*Container, error) {
	req := testcontainers.ContainerRequest{
		Image:        redisImage,
		ExposedPorts: []string{redisPort},
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
	return &Container{Container: container, Address: address}, nil
}
