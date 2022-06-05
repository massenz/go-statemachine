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
	log "github.com/massenz/slf4go/logging"
	"google.golang.org/protobuf/types/known/timestamppb"
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
		listener.logger.Debug("Received event %s", event)
		if event.Destination == "" {
			listener.PostErrorNotification(ErrorMessage(fmt.Errorf("no destination for event"), &event))
			continue
		}
		fsm, ok := listener.store.GetStateMachine(event.Destination)
		if !ok {
			listener.PostErrorNotification(ErrorMessage(fmt.Errorf("statemachine [%s] could not be found",
				event.Destination), &event))
			continue
		}
		// TODO: cache the configuration locally: they are immutable anyway.
		cfg, ok := listener.store.GetConfig(fsm.ConfigId)
		if !ok {
			listener.PostErrorNotification(ErrorMessage(fmt.Errorf("configuration [%s] could not be found",
				fsm.ConfigId), &event))
			continue
		}

		cfgFsm := api.ConfiguredStateMachine{
			Config: cfg,
			FSM:    fsm,
		}
		pbEvent := NewPBEvent(event)
		if err := cfgFsm.SendEvent(pbEvent.Transition.Event); err != nil {
			listener.PostErrorNotification(ErrorMessageWithDetail(err, &event, fmt.Sprintf(
				"FSM [%s] cannot process event `%s`", event.Destination, event.EventName)))
			continue
		}
		err := listener.store.PutStateMachine(event.Destination, fsm)
		if err != nil {
			listener.PostErrorNotification(ErrorMessage(err, &event))
			continue
		}
		listener.logger.Debug("Event %s transitioned FSM [%s] to state `%s`",
			event.EventName, event.Destination, fsm.State)
	}
}

func NewPBEvent(message EventMessage) *api.Event {
	return &api.Event{
		EventId:    message.EventId,
		Originator: message.Sender,
		Timestamp:  timestamppb.New(message.EventTimestamp),
		Transition: &api.Transition{
			Event: message.EventName,
		},
	}
}
