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

package pubsub

import (
    "fmt"
    . "github.com/massenz/go-statemachine/api"
    "github.com/massenz/go-statemachine/storage"
    log "github.com/massenz/slf4go/logging"
    protos "github.com/massenz/statemachine-proto/golang/api"
    "strings"
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

func (listener *EventsListener) PostNotificationAndReportOutcome(errorResponse *protos.EventResponse) {
    if errorResponse.Outcome.Code != protos.EventOutcome_Ok {
        listener.logger.Error("[Event ID: %s]: %s", errorResponse.EventId, errorResponse.GetOutcome().Details)
    }
    if listener.notifications != nil {
        listener.logger.Debug("Posting notification: %v", errorResponse.GetEventId())
        listener.notifications <- *errorResponse
    }
    listener.logger.Debug("Reporting outcome: %v", errorResponse.GetEventId())
    listener.reportOutcome(errorResponse)
}

func (listener *EventsListener) ListenForMessages() {
    listener.logger.Info("Events message listener started")
    for request := range listener.events {
        listener.logger.Debug("Received request %s", request.Event.String())
        if request.Dest == "" {
            listener.PostNotificationAndReportOutcome(makeResponse(&request,
                protos.EventOutcome_MissingDestination,
                fmt.Sprintf("no destination specified")))
            continue
        }
        // TODO: this is an API change and needs to be documented
        // Destination comprises both the type (configuration name) and ID of the statemachine,
        // separated by the # character: <type>#<id> (e.g., `order#1234`)
        dest := strings.Split(request.Dest, "#")
        if len(dest) != 2 {
            listener.PostNotificationAndReportOutcome(makeResponse(&request,
                protos.EventOutcome_MissingDestination,
                fmt.Sprintf("invalid destination [%s] expected <config.name>#<FSM.ID>",
                    request.Dest)))
            continue
        }
        smType, smId := dest[0], dest[1]
        fsm, ok := listener.store.GetStateMachine(smId, smType)
        if !ok {
            listener.PostNotificationAndReportOutcome(makeResponse(&request,
                protos.EventOutcome_FsmNotFound,
                fmt.Sprintf("statemachine [%s] could not be found", request.Dest)))
            continue
        }
        // TODO: cache the configuration locally: they are immutable anyway.
        cfg, ok := listener.store.GetConfig(fsm.ConfigId)
        if !ok {
            listener.PostNotificationAndReportOutcome(makeResponse(&request,
                protos.EventOutcome_ConfigurationNotFound,
                fmt.Sprintf("configuration [%s] could not be found", fsm.ConfigId)))
            continue
        }
        cfgFsm := ConfiguredStateMachine{
            Config: cfg,
            FSM:    fsm,
        }
        if err := cfgFsm.SendEvent(request.Event); err != nil {
            listener.PostNotificationAndReportOutcome(makeResponse(&request,
                protos.EventOutcome_TransitionNotAllowed,
                fmt.Sprintf("event [%s] could not be processed: %v",
                    request.GetEvent().GetTransition().GetEvent(), err)))
            continue
        }
        listener.logger.Info("Event `%s` transitioned FSM [%s] to state `%s` - updating store",
            request.Event.Transition.Event, smId, fsm.State)
        err := listener.store.PutStateMachine(smId, fsm)
        if err != nil {
            listener.PostNotificationAndReportOutcome(makeResponse(&request,
                protos.EventOutcome_InternalError,
                fmt.Sprintf("could not update statemachine [%s] in store: %v",
                    request.Dest, err)))
            continue
        }
        // All good, we want to report success too.
        listener.PostNotificationAndReportOutcome(makeResponse(&request,
            protos.EventOutcome_Ok,
            fmt.Sprintf("event [%s] transitioned FSM [%s] to state [%s]",
                request.Event.Transition.Event, smId, fsm.State)))
    }
}

func (listener *EventsListener) reportOutcome(response *protos.EventResponse) {
    smType := strings.Split(response.Outcome.Dest, "#")[0]
    if err := listener.store.AddEventOutcome(response.EventId, smType, response.Outcome,
        storage.NeverExpire); err != nil {
        listener.logger.Error("could not add outcome to store: %v", err)
    }
}

func makeResponse(request *protos.EventRequest, code protos.EventOutcome_StatusCode,
    details string) *protos.EventResponse {
    return &protos.EventResponse{
        EventId: request.GetEvent().GetEventId(),
        Outcome: &protos.EventOutcome{
            Code:    code,
            Dest:    request.Dest,
            Details: details,
        },
    }
}
