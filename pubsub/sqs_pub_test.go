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
	"github.com/google/uuid"
	"github.com/massenz/slf4go/logging"

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
			testPublisher = pubsub.NewSqsPublisher(notificationsCh, &awsLocal.Address)
			Expect(testPublisher).ToNot(BeNil())
			// Set to DEBUG when diagnosing test failures
			testPublisher.SetLogLevel(logging.NONE)
			SetDefaultEventuallyPollingInterval(pollingInterval)
			SetDefaultEventuallyTimeout(timeout)
		})
		It("can publish error notifications", func() {
			notification := protos.EventResponse{
				EventId: "feed-beef",
				Outcome: &protos.EventOutcome{
					Code:    protos.EventOutcome_InternalError,
					Details: "error details",
					Config:  "test-cfg",
					Id:      "abd-456",
				},
			}
			done := make(chan interface{})
			go func() {
				defer close(done)
				go testPublisher.Publish(getQueueName(notificationsQueue))
			}()
			notificationsCh <- notification
			res := getSqsMessage(getQueueName(notificationsQueue))
			Eventually(res).ShouldNot(BeNil())
			Eventually(res.Body).ShouldNot(BeNil())

			// Emulate SQS Client behavior
			body := *res.Body
			var receivedEvt protos.EventResponse
			Expect(proto.UnmarshalText(body, &receivedEvt)).Should(Succeed())
			Expect(receivedEvt).To(Respect(notification))

			close(notificationsCh)
			Eventually(done).Should(BeClosed())
		})
		It("will not publish successful outcomes", func() {
			notification := protos.EventResponse{
				EventId: "ok-event",
				Outcome: &protos.EventOutcome{
					Code: protos.EventOutcome_Ok,
				},
			}
			done := make(chan interface{})
			go func() {
				defer close(done)
				go testPublisher.Publish(getQueueName(notificationsQueue))
			}()
			notificationsCh <- notification
			Consistently(func(g Gomega) {
				m := getSqsMessage(getQueueName(notificationsQueue))
				g.Expect(m).To(BeNil())
			}).Should(Succeed())
			close(notificationsCh)
			Eventually(done).Should(BeClosed())
		})
		It("will terminate gracefully when the notifications channel is closed", func() {
			done := make(chan interface{})
			go func() {
				defer close(done)
				go testPublisher.Publish(getQueueName(notificationsQueue))
			}()
			close(notificationsCh)
			Eventually(done).Should(BeClosed())
		})
		It("will survive an empty Message", func() {
			go testPublisher.Publish(getQueueName(notificationsQueue))
			notificationsCh <- protos.EventResponse{}
			close(notificationsCh)
			getSqsMessage(getQueueName(notificationsQueue))
		})
		It("will send several messages within a short timeframe", func() {
			go testPublisher.Publish(getQueueName(notificationsQueue))
			for i := range [10]int{} {
				evt := api.NewEvent("do-something")
				evt.EventId = fmt.Sprintf("event-%d", i)
				notificationsCh <- protos.EventResponse{
					EventId: evt.EventId,
					Outcome: &protos.EventOutcome{
						Code:    protos.EventOutcome_InternalError,
						Details: "more details about the error",
						Config:  "test-cfg",
						Id:      fmt.Sprintf("fsm-%d", i),
					},
				}
			}
			done := make(chan interface{})
			go func() {
				// This is necessary as we make assertions in this goroutine,
				// and we want to make sure we can see the notifications if they fail.
				defer GinkgoRecover()
				defer close(done)
				for i := range [10]int{} {
					res := getSqsMessage(getQueueName(notificationsQueue))
					Eventually(res).ShouldNot(BeNil())
					Eventually(res.Body).ShouldNot(BeNil())
					var receivedEvt protos.EventResponse
					Eventually(func(g Gomega) {
						g.Expect(proto.UnmarshalText(*res.Body, &receivedEvt)).Should(Succeed())
						g.Expect(receivedEvt.EventId).To(Equal(fmt.Sprintf("event-%d", i)))
						g.Expect(receivedEvt.Outcome.Code).To(Equal(protos.EventOutcome_InternalError))
						g.Expect(receivedEvt.Outcome.Details).To(Equal("more details about the error"))
						g.Expect(receivedEvt.Outcome.Id).To(ContainSubstring("fsm-"))
					}).Should(Succeed())
				}
			}()
			close(notificationsCh)
			Eventually(done, "5s").Should(BeClosed())
		})
		It("will only notify error outcomes if configured to", func() {
			responseOk := protos.EventResponse{
				EventId: uuid.NewString(),
				Outcome: &protos.EventOutcome{
					Code: protos.EventOutcome_Ok,
				},
			}
			done := make(chan interface{})
			go func() {
				defer close(done)
				go testPublisher.Publish(getQueueName(notificationsQueue))
			}()

			notificationsCh <- responseOk
			Consistently(func(g Gomega) {
				res := getSqsMessage(getQueueName(notificationsQueue))
				Expect(res).To(BeNil())
			}, "200ms").Should(Succeed())
			close(notificationsCh)
			Eventually(done).Should(BeClosed())
		})
	})
})
