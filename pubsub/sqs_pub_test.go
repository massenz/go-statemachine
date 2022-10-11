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
	. "github.com/JiaYongfei/respect/gomega"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/massenz/slf4go/logging"
	"time"

	"github.com/massenz/go-statemachine/api"
	"github.com/massenz/go-statemachine/pubsub"

	protos "github.com/massenz/statemachine-proto/golang/api"
)

var _ = Describe("SQS Publisher", func() {
	Context("when correctly initialized", func() {
		var (
			testPublisher   *pubsub.SqsPublisher
			notificationsCh chan protos.EventResponse
		)
		BeforeEach(func() {
			notificationsCh = make(chan protos.EventResponse)
			ignoreOkOutcomes := false
			testPublisher = pubsub.NewSqsPublisher(notificationsCh, &sqsUrl, ignoreOkOutcomes)
			Expect(testPublisher).ToNot(BeNil())
			// Set to DEBUG when diagnosing test failures
			testPublisher.SetLogLevel(logging.NONE)
		})
		It("can publish error notifications", func() {
			notification := protos.EventResponse{
				EventId: "feed-beef",
				Outcome: &protos.EventOutcome{
					Code:    protos.EventOutcome_InternalError,
					Dest:    "me",
					Details: "error details",
				},
			}
			done := make(chan interface{})
			go func() {
				defer close(done)
				go testPublisher.Publish(getQueueName(notificationsQueue), "")
			}()
			notificationsCh <- notification
			time.Sleep(channelWait)
			res := getSqsMessage(getQueueName(notificationsQueue))
			Expect(res).ToNot(BeNil())
			Expect(res.Body).ToNot(BeNil())

			// Emulate SQS Client behavior
			body := *res.Body
			var receivedEvt protos.EventResponse
			Expect(proto.UnmarshalText(body, &receivedEvt)).Should(Succeed())
			Expect(receivedEvt).To(Respect(notification))

			close(notificationsCh)
			select {
			case <-done:
				Succeed()
			case <-time.After(timeout):
				Fail("timed out waiting for Publisher to exit")
			}
		})
		It("will publish successful outcomes", func() {
			notification := protos.EventResponse{
				EventId: "dead-beef",
				Outcome: &protos.EventOutcome{
					Code: protos.EventOutcome_Ok,
				},
			}
			done := make(chan interface{})
			go func() {
				defer close(done)
				go testPublisher.Publish(getQueueName(notificationsQueue), "")
			}()
			notificationsCh <- notification
			time.Sleep(channelWait)
			m := getSqsMessage(getQueueName(notificationsQueue))
			var response protos.EventResponse
			Expect(proto.UnmarshalText(*m.Body, &response)).ShouldNot(HaveOccurred())
			Expect(&response).To(Respect(&notification))
			close(notificationsCh)

			select {
			case <-done:
				Succeed()
			case <-time.After(100 * time.Millisecond):
				Fail("timed out waiting for Publisher to exit")
			}
		})
		It("will publish OK outcomes to acks queue if configured", func() {
			notification := protos.EventResponse{
				EventId: "dead-pork",
				Outcome: &protos.EventOutcome{
					Code: protos.EventOutcome_InternalError,
				},
			}
			ack := protos.EventResponse{
				EventId: "dead-beef",
				Outcome: &protos.EventOutcome{
					Code: protos.EventOutcome_Ok,
				},
			}
			done := make(chan interface{})
			go func() {
				defer close(done)
				go testPublisher.Publish(getQueueName(notificationsQueue), getQueueName(acksQueue))
			}()
			var response protos.EventResponse

			notificationsCh <- notification
			time.Sleep(channelWait)
			m := getSqsMessage(getQueueName(notificationsQueue))
			Expect(proto.UnmarshalText(*m.Body, &response)).ShouldNot(HaveOccurred())
			Expect(&response).To(Respect(&notification))

			notificationsCh <- ack
			time.Sleep(channelWait)
			n := getSqsMessage(getQueueName(acksQueue))
			Expect(proto.UnmarshalText(*n.Body, &response)).ShouldNot(HaveOccurred())
			Expect(&response).To(Respect(&ack))

			close(notificationsCh)

			select {
			case <-done:
				Succeed()
			case <-time.After(100 * time.Millisecond):
				Fail("timed out waiting for Publisher to exit")
			}
		})
		It("will terminate gracefully when the notifications channel is closed", func() {
			done := make(chan interface{})
			go func() {
				defer close(done)
				go testPublisher.Publish(getQueueName(notificationsQueue), "")
			}()
			close(notificationsCh)
			select {
			case <-done:
				Succeed()
			case <-time.After(timeout):
				Fail("Publisher did not exit within timeout")
			}
		})
		It("will survive an empty Message", func() {
			go testPublisher.Publish(getQueueName(notificationsQueue), "")
			notificationsCh <- protos.EventResponse{}
			close(notificationsCh)
			getSqsMessage(getQueueName(notificationsQueue))
		})
		It("will send several messages within a short timeframe", func() {
			go testPublisher.Publish(getQueueName(notificationsQueue), "")
			for i := range [10]int{} {
				evt := api.NewEvent("do-something")
				evt.EventId = fmt.Sprintf("event-%d", i)
				notificationsCh <- protos.EventResponse{
					EventId: evt.EventId,
					Outcome: &protos.EventOutcome{
						Code:    protos.EventOutcome_InternalError,
						Dest:    fmt.Sprintf("test-%d", i),
						Details: "more details about the error",
					},
				}
			}
			time.Sleep(channelWait * 25)
			done := make(chan interface{})
			go func() {
				// This is necessary as we make assertions in this goroutine,
				// and we want to make sure we can see the notifications if they fail.
				defer GinkgoRecover()
				defer close(done)
				for i := range [10]int{} {
					res := getSqsMessage(getQueueName(notificationsQueue))
					Expect(res).ToNot(BeNil())
					Expect(res.Body).ToNot(BeNil())
					var receivedEvt protos.EventResponse
					Expect(proto.UnmarshalText(*res.Body, &receivedEvt)).Should(Succeed())
					Expect(receivedEvt.EventId).To(Equal(fmt.Sprintf("event-%d", i)))
					Expect(receivedEvt.Outcome.Code).To(Equal(protos.EventOutcome_InternalError))
					Expect(receivedEvt.Outcome.Details).To(Equal("more details about the error"))
					Expect(receivedEvt.Outcome.Dest).To(ContainSubstring("test-"))
				}
			}()
			close(notificationsCh)
			select {
			case <-done:
				Succeed()
			case <-time.After(timeout):
				Fail("timed out waiting for Publisher to exit")
			}
		})
		It("will ignore OK outcomes if configured to", func() {
			ack := protos.EventResponse{
				EventId: "dead-beef",
				Outcome: &protos.EventOutcome{
					Code: protos.EventOutcome_Ok,
				},
			}
			testPublisher = pubsub.NewSqsPublisher(notificationsCh, &sqsUrl, true)
			done := make(chan interface{})
			go func() {
				defer close(done)
				go testPublisher.Publish(getQueueName(notificationsQueue), getQueueName(acksQueue))
			}()

			notificationsCh <- ack
			time.Sleep(channelWait)
			res := getSqsMessage(getQueueName(acksQueue))
			Expect(res).To(BeNil())

			close(notificationsCh)

			select {
			case <-done:
				Succeed()
			case <-time.After(100 * time.Millisecond):
				Fail("timed out waiting for Publisher to exit")
			}
		})
	})
})
