/*
 * Copyright (c) 2022 AlertAvert.com.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Author: Marco Massenzio (marco@alertavert.com)
 */

package pubsub

import (
	"fmt"
	. "github.com/massenz/go-statemachine/api"
	"github.com/massenz/go-statemachine/storage"
	log "github.com/massenz/slf4go/logging"
	protos "github.com/massenz/statemachine-proto/golang/api"
)

func NewEventsListener(options *ListenerOptions) *EventsListener {
	return &EventsListener{
		logger:        log.NewLog("Listener"),
		events:        options.EventsChannel,
		store:         options.StatemachinesStore,
		notifications: options.NotificationsChannel,
	}
}

// SetLogLevel to implement the log.Loggable interface
func (listener *EventsListener) SetLogLevel(level log.LogLevel) {
	listener.logger.Level = level
}

func (listener *EventsListener) PostNotificationAndReportOutcome(eventResponse *protos.EventResponse) {
	if eventResponse.Outcome.Code != protos.EventOutcome_Ok {
		listener.logger.Error("event [%s]: %s",
			eventResponse.GetEventId(), eventResponse.GetOutcome().Details)
	}
	if listener.notifications != nil {
		listener.logger.Debug("posting notification: %v", eventResponse.GetEventId())
		listener.notifications <- *eventResponse
	}
	listener.logger.Debug("Reporting outcome: %v", eventResponse.GetEventId())
	listener.reportOutcome(eventResponse)
}

func (listener *EventsListener) ListenForMessages() {
	listener.logger.Info("Events message listener started")
	for request := range listener.events {
		listener.logger.Debug("Received request %s", request.Event.String())
		fsmId := request.GetId()
		if fsmId == "" {
			listener.PostNotificationAndReportOutcome(makeResponse(&request,
				protos.EventOutcome_MissingDestination,
				fmt.Sprintf("no statemachine ID specified")))
			continue
		}
		config := request.GetConfig()
		if config == "" {
			listener.PostNotificationAndReportOutcome(makeResponse(&request,
				protos.EventOutcome_MissingDestination,
				fmt.Sprintf("no Configuration name specified")))
			continue
		}
		// The event is well-formed, we can store for later retrieval
		if err := listener.store.PutEvent(request.Event, config, storage.NeverExpire); err != nil {
			listener.PostNotificationAndReportOutcome(makeResponse(&request,
				protos.EventOutcome_InternalError,
				fmt.Sprintf("could not store event: %v", err)))
			continue
		}
		fsm, ok := listener.store.GetStateMachine(fsmId, config)
		if !ok {
			listener.PostNotificationAndReportOutcome(makeResponse(&request,
				protos.EventOutcome_FsmNotFound,
				fmt.Sprintf("statemachine [%s] could not be found", fsmId)))
			continue
		}
		// TODO: cache the configuration locally: they are immutable anyway.
		cfg, err := listener.store.GetConfig(fsm.ConfigId)
		if err != nil {
			listener.PostNotificationAndReportOutcome(makeResponse(&request,
				protos.EventOutcome_ConfigurationNotFound,
				fmt.Sprintf("configuration [%s] could not be found", fsm.ConfigId)))
			continue
		}
		previousState := fsm.State
		cfgFsm := ConfiguredStateMachine{
			Config: cfg,
			FSM:    fsm,
		}
		listener.logger.Debug("preparing to send event `%s` for FSM [%s] (current state: %s)",
			request.Event.Transition.Event, fsmId, previousState)
		if err := cfgFsm.SendEvent(request.Event); err != nil {
			listener.PostNotificationAndReportOutcome(makeResponse(&request,
				protos.EventOutcome_TransitionNotAllowed,
				fmt.Sprintf("event [%s] could not be processed: %v",
					request.GetEvent().GetTransition().GetEvent(), err)))
			continue
		}
		if err := listener.store.PutStateMachine(fsmId, fsm); err != nil {
			listener.PostNotificationAndReportOutcome(makeResponse(&request,
				protos.EventOutcome_InternalError,
				fmt.Sprintf("could not update statemachine [%s#%s] in store: %v",
					config, fsmId, err)))
			continue
		}
		if err := listener.store.UpdateState(config, fsmId, previousState, fsm.State); err != nil {
			listener.PostNotificationAndReportOutcome(makeResponse(&request,
				protos.EventOutcome_InternalError,
				fmt.Sprintf("could not update statemachine state set (%s#%s): %v",
					config, fsmId, err)))
			continue
		}
		listener.logger.Debug("Event `%s` transitioned FSM [%s] to state `%s` from state `%s` - updating store",
			request.Event.Transition.Event, fsmId, fsm.State, previousState)
		listener.reportOutcome(makeResponse(&request, protos.EventOutcome_Ok, ""))
	}
}

func (listener *EventsListener) reportOutcome(response *protos.EventResponse) {
	if err := listener.store.AddEventOutcome(response.EventId, response.GetOutcome().GetConfig(),
		response.Outcome, storage.NeverExpire); err != nil {
		listener.logger.Error("could not save event outcome: %v", err)
	}
}

func makeResponse(request *protos.EventRequest, code protos.EventOutcome_StatusCode,
	details string) *protos.EventResponse {
	return &protos.EventResponse{
		EventId: request.GetEvent().GetEventId(),
		Outcome: &protos.EventOutcome{
			Code:    code,
			Details: details,
			Config:  request.Config,
			Id:      request.Id,
		},
	}
}
