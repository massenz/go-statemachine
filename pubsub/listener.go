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
    "bytes"
    "fmt"
    "github.com/massenz/go-statemachine/api"
    "github.com/massenz/go-statemachine/logging"
    "github.com/massenz/go-statemachine/storage"
    "google.golang.org/protobuf/types/known/timestamppb"
)

// An EventsListener will process EventMessage in a separate goroutine.
// The messages are posted on an `events` channel, and if any error is encountered,
// error messages are posted on a `notifications` channel for further processing upstream.
type EventsListener struct {
    logger        *logging.Log
    events        <-chan EventMessage
    notifications chan<- EventErrorMessage
    store         storage.StoreManager
}

// ListenerOptions are used to configure an EventsListener at creation and are used
// to decouple the internals of the listener from its exposed configuration.
type ListenerOptions struct {
    EventsChannel        <-chan EventMessage
    NotificationsChannel chan<- EventErrorMessage
    StatemachinesStore   storage.StoreManager
    ListenersPoolSize    int8
}

func NewEventsListener(options *ListenerOptions) *EventsListener {
    return &EventsListener{
        logger:        logging.NewLog("Listener"),
        events:        options.EventsChannel,
        store:         options.StatemachinesStore,
        notifications: options.NotificationsChannel,
    }
}

// SetLogLevel to implement the logging.Loggable interface
func (l *EventsListener) SetLogLevel(level logging.LogLevel) {
    l.logger.Level = level
}

func (l *EventsListener) PostErrorNotification(msg EventMessage, err error, detail string) {
    var msgBuf bytes.Buffer
    fmt.Fprintf(&msgBuf, "error processing event %s", msg)
    if err != nil {
        fmt.Fprintf(&msgBuf, ": %v", err)
    }
    if detail != "" {
        fmt.Fprintf(&msgBuf, " (%s)", detail)
    }
    l.logger.Error(msgBuf.String())

    var errorMsg = EventErrorMessage{
        Error:       *NewEventProcessingError(err),
        ErrorDetail: detail,
        Message:     &msg,
    }
    if l.notifications != nil {
        l.logger.Debug("Posting notification of error")
        l.notifications <- errorMsg
    }
}

func (l *EventsListener) ListenForMessages() {
    l.logger.Info("Events message listener started")
    for event := range l.events {
        l.logger.Debug("Received event %s", event)
        if event.Destination == "" {
            l.PostErrorNotification(event, fmt.Errorf("no destination for event"), "")
            continue
        }
        fsm, ok := l.store.GetStateMachine(event.Destination)
        if !ok {
            l.PostErrorNotification(event, fmt.Errorf("statemachine [%s] could not be found",
                event.Destination), "")
            continue
        }
        // TODO: cache the configuration locally: they are immutable anyway.
        cfg, ok := l.store.GetConfig(fsm.ConfigId)
        if !ok {
            l.PostErrorNotification(event, fmt.Errorf("configuration [%s] could not be found",
                fsm.ConfigId), "")
            continue
        }

        cfgFsm := api.ConfiguredStateMachine{
            Config: cfg,
            FSM:    fsm,
        }
        pbEvent := NewPBEvent(event)
        if err := cfgFsm.SendEvent(pbEvent.Transition.Event); err != nil {
            l.PostErrorNotification(event, err, fmt.Sprintf(
                "FSM [%s] cannot process event `%s`", event.Destination, event.EventName))
            continue
        }
        err := l.store.PutStateMachine(event.Destination, fsm)
        if err != nil {
            l.PostErrorNotification(event, err, "")
            continue
        }
        l.logger.Debug("Event %s transitioned FSM [%s] to state `%s`",
            event.EventName, event.Destination, fsm.State)
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
