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
			store = storage.NewRedisStoreWithDefaults(redisContainer.Address)
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
				Ω(n.Outcome.Id).To(BeEmpty())
				Ω(n.Outcome.Details).To(Equal(detail))
				Ω(n.Outcome.Code).To(Equal(protos.EventOutcome_MissingDestination))

			case <-time.After(timeout):
				Fail("timed out waiting for notification")
			}
		})
		It("can process well-formed events", func() {
			event := protos.Event{
				EventId:    "feed-beef",
				Originator: "me",
				Transition: &protos.Transition{
					Event: "move",
				},
				Details: "more details",
			}
			request := protos.EventRequest{
				Event:  &event,
				Config: "test",
				Id:     "12345-faa44",
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

			Eventually(func(g Gomega) {
				// Now we want to test that the state machine was updated
				fsm, ok := store.GetStateMachine("12345-faa44", "test")
				g.Ω(ok).ToNot(BeFalse())
				g.Ω(fsm.State).To(Equal("end"))
				g.Ω(len(fsm.History)).To(Equal(1))
				g.Ω(fsm.History[0].Details).To(Equal("more details"))
				g.Ω(fsm.History[0].Transition.Event).To(Equal("move"))
			}).Should(Succeed())
			Eventually(func() bool {
				_, found := store.GetEvent(event.EventId, "test")
				return found
			}).Should(BeTrue())
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
				Event:  &event,
				Config: "test",
				Id:     "fake-fsm",
			}
			go func() {
				testListener.ListenForMessages()
			}()
			eventsCh <- request
			close(eventsCh)
			Eventually(func(g Gomega) {
				select {
				case n := <-notificationsCh:
					g.Ω(n.EventId).To(Equal(request.Event.EventId))
					g.Ω(n.Outcome.Id).To(Equal(request.GetId()))
					g.Ω(n.Outcome.Code).To(Equal(protos.EventOutcome_FsmNotFound))
				}
			}).Should(Succeed())
		})
		It("sends notifications for missing destinations", func() {
			request := protos.EventRequest{
				Event: &protos.Event{
					EventId: "feed-beef",
				},
			}
			go func() { testListener.ListenForMessages() }()
			eventsCh <- request
			close(eventsCh)
			Eventually(func(g Gomega) {
				select {
				case n := <-notificationsCh:
					Ω(n.EventId).To(Equal(request.Event.EventId))
					Ω(n.Outcome).ToNot(BeNil())
					Ω(n.Outcome.Code).To(Equal(protos.EventOutcome_MissingDestination))
				}
			}).Should(Succeed())
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
			Eventually(func() *protos.Event {
				e, _ := store.GetEvent(event.EventId, request.Config)
				return e
			}, 100*time.Millisecond, 20*time.Millisecond).ShouldNot(BeNil())
			Eventually(func() protos.EventOutcome_StatusCode {
				e, ok := store.GetOutcomeForEvent(event.EventId, request.Config)
				if ok {
					return e.Code
				} else {
					return protos.EventOutcome_GenericError
				}
			}, 100*time.Millisecond, 20*time.Millisecond).Should(Equal(protos.EventOutcome_Ok))
		})
	})
})
