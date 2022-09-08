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

func (listener *EventsListener) PostErrorNotification(errorResponse *protos.EventResponse) {
    listener.logger.Error("[Event ID: %s]: %s", errorResponse.EventId, errorResponse.GetOutcome().Details)

    if listener.notifications != nil {
        listener.logger.Debug("Posting notification of error: %v", errorResponse.GetEventId())
        listener.notifications <- *errorResponse
    }
}

func (listener *EventsListener) ListenForMessages() {
    listener.logger.Info("Events message listener started")
    for request := range listener.events {
        listener.logger.Debug("Received request %s", request.Event.String())
        if request.Dest == "" {
            listener.PostErrorNotification(&protos.EventResponse{
                EventId: request.GetEvent().GetEventId(),
                Outcome: &protos.EventOutcome{
                    Code:    protos.EventOutcome_MissingDestination,
                    Details: fmt.Sprintf("no destination specified for event %s", request.GetEvent().GetEventId()),
                },
            })
            continue
        }
        fsm, ok := listener.store.GetStateMachine(request.Dest)
        if !ok {
            listener.PostErrorNotification(&protos.EventResponse{
                EventId: request.GetEvent().GetEventId(),
                Outcome: &protos.EventOutcome{
                    Code:    protos.EventOutcome_FsmNotFound,
                    Details: fmt.Sprintf("statemachine [%s] could not be found", request.Dest),
                },
            })
            continue
        }
        // TODO: cache the configuration locally: they are immutable anyway.
        cfg, ok := listener.store.GetConfig(fsm.ConfigId)
        if !ok {
            listener.PostErrorNotification(&protos.EventResponse{
                EventId: request.GetEvent().GetEventId(),
                Outcome: &protos.EventOutcome{
                    Code: protos.EventOutcome_ConfigurationNotFound,
                    Details: fmt.Sprintf("configuration [%s] could not be found",
                        fsm.ConfigId),
                },
            })
            continue
        }
        cfgFsm := ConfiguredStateMachine{
            Config: cfg,
            FSM:    fsm,
        }
        if err := cfgFsm.SendEvent(request.Event); err != nil {
            listener.PostErrorNotification(&protos.EventResponse{
                EventId: request.GetEvent().GetEventId(),
                Outcome: &protos.EventOutcome{
                    Code: protos.EventOutcome_TransitionNotAllowed,
                    Details: fmt.Sprintf("event [%s] could not be processed: %v",
                        request.GetEvent().GetTransition().GetEvent(), err),
                },
            })
            continue
        }
        err := listener.store.PutStateMachine(request.Dest, fsm)
        if err != nil {
            listener.PostErrorNotification(&protos.EventResponse{
                EventId: request.GetEvent().GetEventId(),
                Outcome: &protos.EventOutcome{
                    Code: protos.EventOutcome_InternalError,
                    Details: fmt.Sprintf("could not update statemachine [%s] in store: %v",
                        request.Dest, err),
                },
            })
            continue
        }
        listener.logger.Info("Event `%s` transitioned FSM [%s] to state `%s`",
            request.Event.Transition.Event, request.Dest, fsm.State)
    }
}
