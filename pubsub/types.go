/*
 * Copyright (c) 2022 AlertAvert.com.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Author: Marco Massenzio (marco@alertavert.com)
 */

package pubsub

import (
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/massenz/go-statemachine/storage"
	log "github.com/massenz/slf4go/logging"
	protos "github.com/massenz/statemachine-proto/golang/api"
	"time"
)

const (
	// DefaultPollingInterval between SQS polling attempts.
	DefaultPollingInterval = 5 * time.Second

	// DefaultVisibilityTimeout sets how long SQS will wait for the subscriber to remove the
	// message from the queue.
	// See: https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/sqs-visibility-timeout.html
	DefaultVisibilityTimeout = 5 * time.Second
)

// An EventsListener will process `EventRequests` in a separate goroutine.
//
// The messages are polled from the `events` channel, and if any error is encountered,
// error messages are posted on a `notifications` channel for further processing upstream.
type EventsListener struct {
	logger        *log.Log
	events        <-chan protos.EventRequest
	notifications chan<- protos.EventResponse
	store         storage.StoreManager
}

// ListenerOptions are used to configure an EventsListener at creation and are used
// to decouple the internals of the listener from its exposed configuration.
type ListenerOptions struct {
	EventsChannel        <-chan protos.EventRequest
	NotificationsChannel chan<- protos.EventResponse
	StatemachinesStore   storage.StoreManager
	ListenersPoolSize    int8
}

// SqsPublisher is a wrapper around the AWS SQS client,
// and is used to publish messages to provided queues when outcomes are encountered.
type SqsPublisher struct {
	logger           *log.Log
	client           *sqs.SQS
	ignoreOkOutcomes bool
	notifications    <-chan protos.EventResponse
}

// SqsSubscriber is a wrapper around the AWS SQS client, and is used to subscribe to Events.
// The subscriber will poll the queue for new messages,
// and will post them on the `events` channel from where an `EventsListener` will process them.
type SqsSubscriber struct {
	logger          *log.Log
	client          *sqs.SQS
	events          chan<- protos.EventRequest
	Timeout         time.Duration
	PollingInterval time.Duration
}
