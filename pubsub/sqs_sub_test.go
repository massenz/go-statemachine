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
    "time"

    . "github.com/JiaYongfei/respect/gomega"
    . "github.com/onsi/ginkgo"
    . "github.com/onsi/gomega"

    "github.com/massenz/go-statemachine/pubsub"
    log "github.com/massenz/slf4go/logging"
)

var _ = Describe("SQS Subscriber", func() {

    Context("when correctly initialized", func() {
        var (
            testSubscriber *pubsub.SqsSubscriber
            eventsCh       chan pubsub.EventMessage
        )
        BeforeEach(func() {
            eventsCh = make(chan pubsub.EventMessage)
            testSubscriber = pubsub.NewSqsSubscriber(eventsCh, &sqsUrl)
            Expect(testSubscriber).ToNot(BeNil())
            // Set to DEBUG when diagnosing failing tests
            testSubscriber.SetLogLevel(log.NONE)
            // Make it exit much sooner in tests
            d, _ := time.ParseDuration("200msec")
            testSubscriber.PollingInterval = d
        })
        It("receives events", func() {
            msg := pubsub.EventMessage{
                Sender:      "me",
                Destination: "some-fsm",
                EventId:     "feed-beef",
                EventName:   "test-me",
            }
            Expect(postSqsMessage(getQueueName(eventsQueue), &msg)).ToNot(HaveOccurred())
            done := make(chan interface{})
            doneTesting := make(chan interface{})
            go func() {
                defer close(done)
                testSubscriber.Subscribe(getQueueName(eventsQueue), doneTesting)
            }()

            select {
            case event := <-eventsCh:
                testLog.Debug("Received Event -- Timestamp: %v", event.EventTimestamp)
                // Workaround as we can't set the time sent
                msg.EventTimestamp = event.EventTimestamp
                Expect(event).To(Respect(msg))
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
