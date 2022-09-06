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

func (listener *EventsListener) PostErrorNotification(error *EventErrorMessage) {
    listener.logger.Error(error.String())
    if listener.notifications != nil {
        listener.logger.Debug("Posting notification of error: %v", *error)
        listener.notifications <- *error
    }
}

func (listener *EventsListener) ListenForMessages() {
    listener.logger.Info("Events message listener started")
    for event := range listener.events {
        listener.logger.Debug("Received event %s", event.Event.String())
        if event.Dest == "" {
            listener.PostErrorNotification(ErrorMessage(fmt.Errorf("no destination for event"),
                event.Event, ""))
            continue
        }
        fsm, ok := listener.store.GetStateMachine(event.Dest)
        if !ok {
            listener.PostErrorNotification(ErrorMessage(
                fmt.Errorf("statemachine [%s] could not be found", event.Dest), event.Event, ""))
            continue
        }
        // TODO: cache the configuration locally: they are immutable anyway.
        cfg, ok := listener.store.GetConfig(fsm.ConfigId)
        if !ok {
            listener.PostErrorNotification(ErrorMessage(
                fmt.Errorf("configuration [%s] could not be found",
                    fsm.ConfigId), event.Event, ""))
            continue
        }

        cfgFsm := ConfiguredStateMachine{
            Config: cfg,
            FSM:    fsm,
        }
        if err := cfgFsm.SendEvent(event.Event); err != nil {
            listener.PostErrorNotification(ErrorMessage(err, event.Event, fmt.Sprintf(
                "FSM [%s] cannot process event `%s`", event.Dest, event.Event.Transition.Event)))
            continue
        }
        err := listener.store.PutStateMachine(event.Dest, fsm)
        if err != nil {
            listener.PostErrorNotification(ErrorMessage(err, event.Event, "could not save FSM"))
            continue
        }
        listener.logger.Info("Event `%s` transitioned FSM [%s] to state `%s`",
            event.Event.Transition.Event, event.Dest, fsm.State)
    }
}
