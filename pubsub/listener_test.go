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
    Context("when store-backed", func() {
        var (
            testListener    *pubsub.EventsListener
            eventsCh        chan api.EventRequest
            notificationsCh chan pubsub.EventErrorMessage
            store           storage.StoreManager
        )
        BeforeEach(func() {
            eventsCh = make(chan api.EventRequest)
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
            msg := api.Event{
                EventId:    "feed-beef",
                Originator: "me",
                Transition: &api.Transition{
                    Event: "test-me",
                },
                Details: "more details",
            }
            detail := "some error"
            notification := pubsub.ErrorMessage(fmt.Errorf("this is a test"), &msg, detail)
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
            event := api.Event{
                EventId:    "feed-beef",
                Originator: "me",
                Transition: &api.Transition{
                    Event: "move",
                },
                Details: "more details",
            }
            request := api.EventRequest{
                Event: &event,
                Dest:  "test-fsm",
            }
            Expect(store.PutStateMachine(request.Dest, &api.FiniteStateMachine{
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
            eventsCh <- request
            close(eventsCh)

            select {
            case n := <-notificationsCh:
                Fail(fmt.Sprintf("unexpected error: %v", n.String()))
            case <-done:
                fsm, ok := store.GetStateMachine(request.Dest)
                Expect(ok).ToNot(BeFalse())
                Expect(fsm.State).To(Equal("end"))
                Expect(len(fsm.History)).To(Equal(1))
                Expect(fsm.History[0].Details).To(Equal("more details"))
                Expect(fsm.History[0].Transition.Event).To(Equal("move"))
            case <-time.After(timeout):
                Fail("the listener did not exit when the events channel was closed")
            }
        })
        It("sends notifications for missing statemachine", func() {
            event := api.Event{
                EventId:    "feed-beef",
                Originator: "me",
                Transition: &api.Transition{
                    Event: "move",
                },
                Details: "more details",
            }
            request := api.EventRequest{
                Event: &event,
                Dest:  "fake-fsm",
            }
            go func() {
                testListener.ListenForMessages()
            }()
            eventsCh <- request
            close(eventsCh)

            select {
            case n := <-notificationsCh:
                Expect(n.Message).ToNot(BeNil())
                Expect(n.Message.EventId).To(Equal(request.Event.EventId))
                Expect(n.Error.Error()).To(Equal("statemachine [fake-fsm] could not be found"))
            case <-time.After(timeout):
                Fail("the listener did not exit when the events channel was closed")
            }
        })
        It("sends notifications for missing destinations", func() {
            request := api.EventRequest{
                Event: &api.Event{
                    EventId: "feed-beef",
                },
                Dest: "",
            }

            go func() {
                testListener.ListenForMessages()
            }()
            eventsCh <- request
            close(eventsCh)

            select {
            case n := <-notificationsCh:
                Expect(n.Message).ToNot(BeNil())
                Expect(n.Message.EventId).To(Equal(request.Event.EventId))
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
