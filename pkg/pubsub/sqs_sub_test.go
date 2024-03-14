/*
 * Copyright (c) 2022 AlertAvert.com.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Author: Marco Massenzio (marco@alertavert.com)
 */

package pubsub_test

import (
	"github.com/massenz/go-statemachine/pkg/api"
	pubsub2 "github.com/massenz/go-statemachine/pkg/pubsub"
	"time"

	. "github.com/JiaYongfei/respect/gomega"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	log "github.com/massenz/slf4go/logging"

	protos "github.com/massenz/statemachine-proto/golang/api"
)

var _ = Describe("SQS Subscriber", func() {
	Context("when correctly initialized", func() {
		var (
			testSubscriber *pubsub2.SqsSubscriber
			eventsCh       chan protos.EventRequest
		)
		BeforeEach(func() {
			Expect(awsLocal).ToNot(BeNil())
			eventsCh = make(chan protos.EventRequest)
			testSubscriber = pubsub2.NewSqsSubscriber(eventsCh, &awsLocal.Address)
			Expect(testSubscriber).ToNot(BeNil())
			// Set to DEBUG when diagnosing failing tests
			testSubscriber.SetLogLevel(log.NONE)
			// Make it exit much sooner in tests
			d, _ := time.ParseDuration("200msec")
			testSubscriber.PollingInterval = d
		})
		It("receives events", func() {
			msg := protos.EventRequest{
				Event: api.NewEvent("test-event"),
				Id:    "some-fsm",
			}
			msg.Event.EventId = "feed-beef"
			msg.Event.Originator = "test-subscriber"
			Expect(postSqsMessage(getQueueName(eventsQueue), &msg)).Should(Succeed())
			done := make(chan interface{})
			doneTesting := make(chan interface{})
			go func() {
				defer close(done)
				testSubscriber.Subscribe(getQueueName(eventsQueue), doneTesting)
			}()

			select {
			case req := <-eventsCh:
				// We null the timestamp as we don't want to compare that with Respect
				msg.Event.Timestamp = nil
				req.Event.Timestamp = nil
				Expect(req.Event).To(Respect(msg.Event))
				close(doneTesting)
			case <-time.After(timeout):
				Fail("timed out waiting to receive a message")
			}

			select {
			case <-done:
				Succeed()
			case <-time.After(timeout):
				Fail("timed out waiting for the subscriber to exit")
			}
		})
	})
})
