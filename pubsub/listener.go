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
    "github.com/massenz/go-statemachine/api"
    "github.com/massenz/go-statemachine/logging"
    "github.com/massenz/go-statemachine/storage"
    "google.golang.org/protobuf/types/known/timestamppb"
)

type EventsListener struct {
    logger *logging.Log
    events <-chan EventMessage
    store  storage.StoreManager
}

func NewEventsListener(store storage.StoreManager, events <-chan EventMessage) *EventsListener {
    return &EventsListener{
        logger: logging.NewLog("Listener"),
        events: events,
        store:  store,
    }
}

func (l *EventsListener) SetLogLevel(level logging.LogLevel) {
    l.logger.Level = level
}

func (l *EventsListener) PostErrorNotification(msg EventMessage, errMsg string) {
    l.logger.Error("error processing event [%s]: %s", msg.String(), errMsg)
    // TODO: post to the notifications channels, to be eventually posted to the DLQ
}

func (l *EventsListener) ListenForMessages() {
    l.logger.Info("Events message listener started")
    for event := range l.events {
        l.logger.Debug("Received event %s", event)
        if event.Destination == "" {
            l.PostErrorNotification(event, fmt.Sprintf("No destination for event %s", event.EventId))
            continue
        }
        fsm, ok := l.store.GetStateMachine(event.Destination)
        if !ok {
            l.PostErrorNotification(event, fmt.Sprintf("StateMachine [%s] could not be found",
                event.Destination))
            continue
        }
        // TODO: cache the configuration locally: they are immutable anyway.
        cfg, ok := l.store.GetConfig(fsm.ConfigId)
        if !ok {
            l.PostErrorNotification(event, fmt.Sprintf("Configuration [%s] could not be found",
                fsm.ConfigId))
            continue
        }

        cfgFsm := api.ConfiguredStateMachine{
            Config: cfg,
            FSM:    fsm,
        }
        pbEvent := NewPBEvent(event)
        if err := cfgFsm.SendEvent(pbEvent.Transition.Event); err != nil {
            l.PostErrorNotification(event, fmt.Sprintf("Cannot send event [%s]: %v",
                event.String(), err))
            continue
        }
        err := l.store.PutStateMachine(event.Destination, fsm)
        if err != nil {
            l.PostErrorNotification(event, err.Error())
            continue
        }
        l.logger.Debug("Event %s for FSM %s processed", event.String(), fsm.String())
    }
}

func NewPBEvent(message EventMessage) *api.Event {
    return &api.Event{
        EventId:   message.EventId,
        Timestamp: timestamppb.New(message.EventTimestamp),
        Transition: &api.Transition{
            Event: message.EventName,
        },
    }
}
