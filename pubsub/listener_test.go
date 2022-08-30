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
    "github.com/massenz/slf4go/logging"
    "time"

    "github.com/massenz/go-statemachine/pubsub"
    "github.com/massenz/go-statemachine/storage"
    "github.com/massenz/statemachine-proto/golang/api"
)

var _ = Describe("A Listener", func() {
    Context("from an EventMessage", func() {
        It("can create a PB Event", func() {
            msg := pubsub.EventMessage{
                Sender:      "test-sender",
                Destination: "test-destination",
                EventId:     "test-abed",
                EventName:   "an-event",
                // 2022-05-09T22:52:39+0000
                EventTimestamp: time.Unix(1652161959, 0),
            }
            evt := pubsub.NewPBEvent(msg)
            Expect(evt).ToNot(BeNil())
            Expect(evt.EventId).To(Equal(msg.EventId))
            Expect(evt.Transition).ToNot(BeNil())
            Expect(evt.Transition.Event).To(Equal(msg.EventName))
            Expect(evt.Timestamp.AsTime().Unix()).To(Equal(msg.EventTimestamp.Unix()))
            Expect(evt.Originator).To(Equal(msg.Sender))
        })
    })

    Context("when store-backed", func() {
        var (
            testListener    *pubsub.EventsListener
            eventsCh        chan pubsub.EventMessage
            notificationsCh chan pubsub.EventErrorMessage
            store           storage.StoreManager
        )
        BeforeEach(func() {
            eventsCh = make(chan pubsub.EventMessage)
            notificationsCh = make(chan pubsub.EventErrorMessage)
            store = storage.NewInMemoryStore()
            testListener = pubsub.NewEventsListener(&pubsub.ListenerOptions{
                EventsChannel:        eventsCh,
                NotificationsChannel: notificationsCh,
                StatemachinesStore:   store,
                ListenersPoolSize:    0,
            })
            // Set to DEBUG when diagnosing test failures
            testListener.SetLogLevel(logging.NONE)
        })
        It("can post error notifications", func() {
            defer close(notificationsCh)
            msg := pubsub.EventMessage{
                Sender:    "me",
                EventId:   "feed-beef",
                EventName: "test-me",
            }
            detail := "more details about the error"
            notification := pubsub.ErrorMessageWithDetail(fmt.Errorf("this is a test"), &msg, detail)
            go testListener.PostErrorNotification(notification)
            select {
            case n := <-notificationsCh:
                Expect(n.Error.Error()).To(Equal("this is a test"))
                Expect(n.Message).ToNot(BeNil())
                Expect(*n.Message).To(Respect(msg))

            case <-time.After(timeout):
                Fail("timed out waiting for notification")
            }
        })
        It("can receive events", func() {
            done := make(chan interface{})
            msg := pubsub.EventMessage{
                Sender:      "1234",
                EventId:     "feed-dead-beef",
                EventName:   "move",
                Destination: "99",
            }
            Expect(store.PutStateMachine(msg.Destination, &api.FiniteStateMachine{
                ConfigId: "test:v1",
                State:    "start",
                History:  nil,
            })).ToNot(HaveOccurred())
            Expect(store.PutConfig(&api.Configuration{
                Name:          "test",
                Version:       "v1",
                States:        []string{"start", "end"},
                Transitions:   []*api.Transition{{From: "start", To: "end", Event: "move"}},
                StartingState: "start",
            })).ToNot(HaveOccurred())

            go func() {
                defer close(done)
                testListener.ListenForMessages()
            }()
            eventsCh <- msg
            close(eventsCh)

            select {
            case n := <-notificationsCh:
                Fail(fmt.Sprintf("unexpected error: %v", n.String()))
            case <-done:
                fsm, ok := store.GetStateMachine(msg.Destination)
                Expect(ok).ToNot(BeFalse())
                Expect(fsm.State).To(Equal("end"))
            case <-time.After(timeout):
                Fail("the listener did not exit when the events channel was closed")
            }
        })
        It("sends notifications for missing configurations", func() {
            msg := pubsub.EventMessage{
                Sender:      "1234",
                EventId:     "feed-beef",
                EventName:   "move",
                Destination: "778899",
            }
            Expect(store.PutStateMachine(msg.Destination, &api.FiniteStateMachine{
                ConfigId: "test.v3",
                State:    "start",
                History:  nil,
            })).ToNot(HaveOccurred())
            go func() {
                testListener.ListenForMessages()
            }()
            eventsCh <- msg
            close(eventsCh)

            select {
            case n := <-notificationsCh:
                Expect(n.Message).ToNot(BeNil())
                Expect(n.Message.EventId).To(Equal(msg.EventId))
                Expect(n.Error.Error()).To(Equal("configuration [test.v3] could not be found"))
            case <-time.After(timeout):
                Fail("the listener did not exit when the events channel was closed")
            }
        })
        It("sends notifications for missing FSM", func() {
            msg := pubsub.EventMessage{
                Sender:      "1234",
                EventId:     "feed-beef",
                EventName:   "failed",
                Destination: "fake",
            }
            go func() {
                testListener.ListenForMessages()
            }()
            eventsCh <- msg
            close(eventsCh)

            select {
            case n := <-notificationsCh:
                Expect(n.Message).ToNot(BeNil())
                Expect(n.Message.EventId).To(Equal(msg.EventId))
                Expect(n.Error.Error()).To(Equal("statemachine [fake] could not be found"))

            case <-time.After(timeout):
                Fail("no error notification received")
            }
        })
        It("sends notifications for missing destinations", func() {
            msg := pubsub.EventMessage{
                Sender:    "1234",
                EventId:   "feed-beef",
                EventName: "failed",
            }
            go func() {
                testListener.ListenForMessages()
            }()
            eventsCh <- msg
            close(eventsCh)

            select {
            case n := <-notificationsCh:
                Expect(n.Message).ToNot(BeNil())
                Expect(n.Message.EventId).To(Equal(msg.EventId))
                Expect(n.Error.Error()).To(Equal("no destination for event"))
            case <-time.After(timeout):
                Fail("no error notification received")
            }
        })
        It("should exit when the channel is closed", func() {
            done := make(chan interface{})
            go func() {
                defer close(done)
                testListener.ListenForMessages()
            }()
            close(eventsCh)
            select {
            case <-done:
                Succeed()
            case <-time.After(timeout):
                Fail("the listener did not exit when the events channel was closed")
            }
        })
    })
})
