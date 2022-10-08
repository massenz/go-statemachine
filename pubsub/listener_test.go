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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/massenz/slf4go/logging"
	"time"

	"github.com/massenz/go-statemachine/pubsub"
	"github.com/massenz/go-statemachine/storage"
	protos "github.com/massenz/statemachine-proto/golang/api"
)

var _ = Describe("A Listener", func() {
	Context("when store-backed", func() {
		var (
			testListener    *pubsub.EventsListener
			eventsCh        chan protos.EventRequest
			notificationsCh chan protos.EventResponse
			store           storage.StoreManager
		)
		BeforeEach(func() {
			eventsCh = make(chan protos.EventRequest)
			notificationsCh = make(chan protos.EventResponse)
			store = storage.NewInMemoryStore()
			store.SetLogLevel(logging.NONE)
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
			msg := protos.Event{
				EventId:    "feed-beef",
				Originator: "me",
				Transition: &protos.Transition{
					Event: "test-me",
				},
				Details: "more details",
			}
			detail := "some error"
			notification := &protos.EventResponse{
				EventId: msg.GetEventId(),
				Outcome: &protos.EventOutcome{
					Code:    protos.EventOutcome_MissingDestination,
					Details: detail,
				},
			}
			go testListener.PostNotificationAndReportOutcome(notification)
			select {
			case n := <-notificationsCh:
				Ω(n.EventId).To(Equal(msg.GetEventId()))
				Ω(n.Outcome).ToNot(BeNil())
				Ω(n.Outcome.Dest).To(BeEmpty())
				Ω(n.Outcome.Details).To(Equal(detail))
				Ω(n.Outcome.Code).To(Equal(protos.EventOutcome_MissingDestination))

			case <-time.After(timeout):
				Fail("timed out waiting for notification")
			}
		})
		It("can receive events", func() {
			event := protos.Event{
				EventId:    "feed-beef",
				Originator: "me",
				Transition: &protos.Transition{
					Event: "move",
				},
				Details: "more details",
			}
			request := protos.EventRequest{
				Event: &event,
				Dest:  "test#12345-faa44",
			}
			Ω(store.PutConfig(&protos.Configuration{
				Name:          "test",
				Version:       "v1",
				States:        []string{"start", "end"},
				Transitions:   []*protos.Transition{{From: "start", To: "end", Event: "move"}},
				StartingState: "start",
			})).ToNot(HaveOccurred())
			Ω(store.PutStateMachine("12345-faa44", &protos.FiniteStateMachine{
				ConfigId: "test:v1",
				State:    "start",
				History:  nil,
			})).ToNot(HaveOccurred())

			go func() {
				testListener.ListenForMessages()
			}()
			eventsCh <- request
			close(eventsCh)

			select {
			case notification := <-notificationsCh:
				// First we want to test that the outcome was successful
				Ω(notification.EventId).To(Equal(event.GetEventId()))
				Ω(notification.Outcome).ToNot(BeNil())
				Ω(notification.Outcome.Dest).To(Equal(request.GetDest()))
				Ω(notification.Outcome.Details).To(ContainSubstring("transitioned"))
				Ω(notification.Outcome.Code).To(Equal(protos.EventOutcome_Ok))

				// Now we want to test that the state machine was updated
				fsm, ok := store.GetStateMachine("12345-faa44", "test")
				Ω(ok).ToNot(BeFalse())
				Ω(fsm.State).To(Equal("end"))
				Ω(len(fsm.History)).To(Equal(1))
				Ω(fsm.History[0].Details).To(Equal("more details"))
				Ω(fsm.History[0].Transition.Event).To(Equal("move"))
			case <-time.After(timeout):
				Fail("the listener did not exit when the events channel was closed")
			}
		})
		It("sends notifications for missing state-machine", func() {
			event := protos.Event{
				EventId:    "feed-beef",
				Originator: "me",
				Transition: &protos.Transition{
					Event: "move",
				},
				Details: "more details",
			}
			request := protos.EventRequest{
				Event: &event,
				Dest:  "test#fake-fsm",
			}
			go func() {
				testListener.ListenForMessages()
			}()
			eventsCh <- request
			close(eventsCh)
			select {
			case n := <-notificationsCh:
				Ω(n.EventId).To(Equal(request.Event.EventId))
				Ω(n.Outcome).ToNot(BeNil())
				Ω(n.Outcome.Dest).To(Equal(request.Dest))
				Ω(n.Outcome.Code).To(Equal(protos.EventOutcome_FsmNotFound))
			case <-time.After(timeout):
				Fail("the listener did not exit when the events channel was closed")
			}
		})
		It("sends notifications for missing destinations", func() {
			request := protos.EventRequest{
				Event: &protos.Event{
					EventId: "feed-beef",
				},
				Dest: "",
			}
			go func() { testListener.ListenForMessages() }()
			eventsCh <- request
			close(eventsCh)

			select {
			case n := <-notificationsCh:
				Ω(n.EventId).To(Equal(request.Event.EventId))
				Ω(n.Outcome).ToNot(BeNil())
				Ω(n.Outcome.Code).To(Equal(protos.EventOutcome_MissingDestination))
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
		It("can post ok outcomes and error notifications", func() {
			// Prepare for outcomes
			outcomesCh := make(chan protos.EventResponse)
			testListener = pubsub.NewEventsListener(&pubsub.ListenerOptions{
				EventsChannel:        eventsCh,
				NotificationsChannel: notificationsCh,
				OutcomesChannel:      outcomesCh,
				StatemachinesStore:   store,
				ListenersPoolSize:    0,
			})

			defer close(outcomesCh)
			msg := protos.Event{
				EventId:    "feed-beef",
				Originator: "me",
				Transition: &protos.Transition{
					Event: "test-me",
				},
				Details: "more details",
			}
			detail := "some error"
			notification := &protos.EventResponse{
				EventId: msg.GetEventId(),
				Outcome: &protos.EventOutcome{
					Code:    protos.EventOutcome_MissingDestination,
					Details: detail,
				},
			}
			outcome := &protos.EventResponse{
				EventId: msg.GetEventId(),
				Outcome: &protos.EventOutcome{
					Code:    protos.EventOutcome_Ok,
					Details: detail,
				},
			}
			go testListener.PostNotificationAndReportOutcome(notification)
			go testListener.PostNotificationAndReportOutcome(outcome)
			select {
			case n := <-notificationsCh:
				Ω(n.EventId).To(Equal(msg.GetEventId()))
				Ω(n.Outcome).ToNot(BeNil())
				Ω(n.Outcome.Dest).To(BeEmpty())
				Ω(n.Outcome.Details).To(Equal(detail))
				Ω(n.Outcome.Code).To(Equal(protos.EventOutcome_MissingDestination))
			case o := <-outcomesCh:
				Ω(o.EventId).To(Equal(msg.GetEventId()))
				Ω(o.Outcome).ToNot(BeNil())
				Ω(o.Outcome.Dest).To(BeEmpty())
				Ω(o.Outcome.Details).To(Equal(detail))
				Ω(o.Outcome.Code).To(Equal(protos.EventOutcome_Ok))

			case <-time.After(timeout):
				Fail("timed out waiting for notification")
			}
		})
	})
})
