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
	storage2 "github.com/massenz/go-statemachine/pkg/storage"
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
		cfgName := request.GetConfig()
		if cfgName == "" {
			listener.PostNotificationAndReportOutcome(makeResponse(&request,
				protos.EventOutcome_MissingDestination,
				fmt.Sprintf("no Configuration name specified")))
			continue
		}
		// The event is well-formed, we can store for later retrieval
		if err := listener.store.PutEvent(request.Event, cfgName, storage2.NeverExpire); err != nil {
			listener.PostNotificationAndReportOutcome(makeResponse(&request,
				protos.EventOutcome_InternalError,
				fmt.Sprintf("could not store event: %v", err)))
			continue
		}
		listener.logger.Debug("preparing to send event `%s` for FSM [%s]",
			request.Event.Transition.Event, fsmId)
		if err := listener.store.TxProcessEvent(fsmId, cfgName, request.Event); err != nil {
			var errCode protos.EventOutcome_StatusCode
			if storage2.IsNotFoundErr(err) {
				errCode = protos.EventOutcome_FsmNotFound
			} else {
				errCode = protos.EventOutcome_InternalError
			}
			listener.PostNotificationAndReportOutcome(makeResponse(&request,
				errCode,
				fmt.Sprintf("could not update statemachine [%s#%s] in store: %v",
					cfgName, fsmId, err)))
			continue
		}
		listener.logger.Debug("Event `%s` successfully changed FSM [%s] state",
			request.Event.Transition.Event, fsmId)
		listener.reportOutcome(makeResponse(&request, protos.EventOutcome_Ok, ""))
	}
}

func (listener *EventsListener) reportOutcome(response *protos.EventResponse) {
	if err := listener.store.AddEventOutcome(response.EventId, response.GetOutcome().GetConfig(),
		response.Outcome, storage2.NeverExpire); err != nil {
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
