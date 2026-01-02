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
	"time"

	. "github.com/JiaYongfei/respect/gomega"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rs/zerolog"
	"github.com/massenz/go-statemachine/pkg/pubsub"
	"github.com/massenz/go-statemachine/pkg/storage"

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
			store = storage.NewRedisStoreWithDefaults(redisContainer.Address)
			zerolog.SetGlobalLevel(zerolog.Disabled)
			testListener = pubsub.NewEventsListener(&pubsub.ListenerOptions{
				EventsChannel:        eventsCh,
				NotificationsChannel: notificationsCh,
				StatemachinesStore:   store,
				ListenersPoolSize:    0,
			})
			// Logs are globally muted above; bump level during debugging if needed.
		})
		const eventId = "1234-abcdef"
		It("can post error notifications", func() {
			defer close(notificationsCh)
			msg := protos.Event{
				EventId:    eventId,
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
				Ω(n.Outcome.Id).To(BeEmpty())
				Ω(n.Outcome.Details).To(Equal(detail))
				Ω(n.Outcome.Code).To(Equal(protos.EventOutcome_MissingDestination))

			case <-time.After(timeout):
				Fail("timed out waiting for notification")
			}
		})
		It("can process well-formed events", func() {
			event := protos.Event{
				EventId:    eventId,
				Originator: "me",
				Transition: &protos.Transition{
					Event: "move",
				},
				Details: "more details",
			}
			const requestId = "12345-faa44"
			request := protos.EventRequest{
				Event:  &event,
				Config: "test",
				Id:     requestId,
			}
			Ω(store.PutConfig(&protos.Configuration{
				Name:          "test",
				Version:       "v1",
				States:        []string{"start", "end"},
				Transitions:   []*protos.Transition{{From: "start", To: "end", Event: "move"}},
				StartingState: "start",
			})).ToNot(HaveOccurred())
			Ω(store.PutStateMachine(requestId, &protos.FiniteStateMachine{
				ConfigId: "test:v1",
				State:    "start",
				History:  nil,
			})).ToNot(HaveOccurred())

			go func() {
				testListener.ListenForMessages()
			}()
			eventsCh <- request
			close(eventsCh)

			Eventually(func(g Gomega) {
				// Now we want to test that the state machine was updated
				fsm, err := store.GetStateMachine(requestId, "test")
				g.Ω(err).To(BeNil())
				g.Ω(fsm.State).To(Equal("end"))
				g.Ω(len(fsm.History)).To(Equal(1))
				g.Ω(fsm.History[0].Details).To(Equal("more details"))
				g.Ω(fsm.History[0].Transition.Event).To(Equal("move"))
			}, 120*time.Millisecond, 40*time.Millisecond).Should(Succeed())
			Eventually(func() storage.StoreErr {
				_, err := store.GetEvent(event.EventId, "test")
				return err
			}).Should(BeNil())
		})
		It("sends notifications for missing state-machine", func() {
			event := protos.Event{
				EventId:    eventId,
				Originator: "me",
				Transition: &protos.Transition{
					Event: "fake",
				},
			}
			request := protos.EventRequest{
				Event:  &event,
				Config: "test",
				Id:     "fake-fsm",
			}
			go func() {
				testListener.ListenForMessages()
			}()
			eventsCh <- request
			close(eventsCh)
			select {
			case n := <-notificationsCh:
				Ω(n.EventId).To(Equal(request.Event.EventId))
				Ω(n.Outcome.Id).To(Equal(request.GetId()))
				Ω(n.Outcome.Code).To(Equal(protos.EventOutcome_FsmNotFound))
			case <-time.After(timeout):
				Fail("timed out waiting for notification")
			}
		})
		It("sends notifications for missing destinations", func() {
			request := protos.EventRequest{
				Event: &protos.Event{
					EventId: eventId,
				},
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
				Fail("timed out waiting for notification")
			}
		})
		It("should exit when the channel is closed", func() {
			done := make(chan interface{})
			go func() {
				defer close(done)
				testListener.ListenForMessages()
			}()
			close(eventsCh)
			Eventually(done).Should(BeClosed())
		})
		It("should store a successful event and outcome (but no notification)", func() {
			event := protos.Event{
				EventId:    "1234",
				Originator: "test-ok",
				Transition: &protos.Transition{
					Event: "move",
				},
			}
			request := protos.EventRequest{
				Event:  &event,
				Config: "test",
				Id:     "1244",
			}
			Ω(store.PutConfig(&protos.Configuration{
				Name:          "test",
				Version:       "v2",
				States:        []string{"start", "end"},
				Transitions:   []*protos.Transition{{From: "start", To: "end", Event: "move"}},
				StartingState: "start",
			})).ToNot(HaveOccurred())
			Ω(store.PutStateMachine("1244", &protos.FiniteStateMachine{
				ConfigId: "test:v2",
				State:    "start",
				History:  nil,
			})).ToNot(HaveOccurred())
			go func() { testListener.ListenForMessages() }()
			eventsCh <- request
			close(eventsCh)
			Consistently(func() *protos.EventResponse {
				select {
				case n := <-notificationsCh:
					return &n
				default:
					return nil
				}
			}).Should(BeNil())
			Eventually(func(g Gomega) {
				evt, err := store.GetEvent(event.EventId, request.Config)
				Ω(err).ToNot(HaveOccurred())
				if evt != nil {
					Ω(evt).To(Respect(&event))
				} else {
					Fail("event is nil")
				}
			}, 100*time.Millisecond, 20*time.Millisecond).Should(Succeed())
			Eventually(func(g Gomega) {
				outcome, err := store.GetOutcomeForEvent(event.EventId, request.Config)
				Ω(err).ToNot(HaveOccurred())
				if outcome != nil {
					Ω(outcome.Code).To(Equal(protos.EventOutcome_Ok))
				}
			}, 100*time.Millisecond, 20*time.Millisecond).Should(Succeed())
		})
	})
})
