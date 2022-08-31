/*
 * Copyright (c) 2022 AlertAvert.com.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
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
    log "github.com/massenz/slf4go/logging"
    "time"

    "github.com/massenz/go-statemachine/api"
    "github.com/massenz/go-statemachine/pubsub"

    protos "github.com/massenz/statemachine-proto/golang/api"
)

var _ = Describe("SQS Publisher", func() {
    Context("when correctly initialized", func() {
        var (
            testPublisher   *pubsub.SqsPublisher
            notificationsCh chan pubsub.EventErrorMessage
        )
        BeforeEach(func() {
            notificationsCh = make(chan pubsub.EventErrorMessage)
            testPublisher = pubsub.NewSqsPublisher(notificationsCh, &sqsUrl)
            Expect(testPublisher).ToNot(BeNil())
            // Set to DEBUG when diagnosing test failures
            testPublisher.SetLogLevel(log.NONE)
        })
        It("can publish error notifications", func() {
            msg := api.NewEvent("test-event")
            msg.Originator = "me"
            msg.EventId = "feed-beef"
            msg.Details = `{"foo": "bar"}`
            detail := "more details about the error"
            notification := pubsub.ErrorMessage(fmt.Errorf("this is a test"), msg, detail)
            done := make(chan interface{})
            go func() {
                defer close(done)
                go testPublisher.Publish(getQueueName(notificationsQueue))

            }()
            notificationsCh <- *notification
            res := getSqsMessage(getQueueName(notificationsQueue))
            Expect(res).ToNot(BeNil())

            body := *res.Body
            var receivedEvt protos.Event
            Expect(proto.UnmarshalText(body, &receivedEvt)).Should(Succeed())
            Expect(receivedEvt).To(Respect(*msg))

            close(notificationsCh)
            select {
            case <-done:
                Succeed()
            case <-time.After(timeout):
                Fail("timed out waiting for Publisher to exit")
            }
        })

        It("will terminate gracefully when the notifications channel is closed", func() {
            done := make(chan interface{})
            go func() {
                defer close(done)
                testPublisher.Publish(getQueueName(notificationsQueue))
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
            go testPublisher.Publish(getQueueName(notificationsQueue))
            notificationsCh <- pubsub.EventErrorMessage{}
            close(notificationsCh)
            getSqsMessage(getQueueName(notificationsQueue))
        })

        It("will send several messages within a reasonable timeframe", func() {
            go testPublisher.Publish(getQueueName(notificationsQueue))
            for range [10]int{} {
                evt := api.NewEvent("many-messages-test")
                detail := "more details about the error"
                notificationsCh <- *pubsub.ErrorMessage(fmt.Errorf("this is a test"), evt, detail)
            }
            done := make(chan interface{})
            go func() {
                // This is necessary as we make assertions in this goroutine,
                //and we want to make sure we can see the errors if they fail.
                defer GinkgoRecover()
                defer close(done)
                for range [10]int{} {
                    res := getSqsMessage(getQueueName(notificationsQueue))
                    Expect(res).ToNot(BeNil())
                    var receivedEvt protos.Event
                    Expect(proto.UnmarshalText(*res.Body, &receivedEvt)).Should(Succeed())
                    Expect(receivedEvt.Transition.Event).To(Equal("many-messages-test"))
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
    })
})
