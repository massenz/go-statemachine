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
    "encoding/json"
    "fmt"
    "github.com/massenz/go-statemachine/logging"
    "github.com/massenz/go-statemachine/storage"
    "time"
)

// EventMessage abstracts away the details of the actual structure of the events and the actual
// message broker implementation.  It is the Internal Representation (
// IR) for an event being originated by the `sender` and being sent to a `Destination` StateMachine.
type EventMessage struct {
    Sender         string    `json:"sender,omitempty"`
    Destination    string    `json:"destination,omitempty"`
    EventId        string    `json:"event_id,omitempty"`
    EventName      string    `json:"event_name,omitempty"`
    EventTimestamp time.Time `json:"timestamp"`
}

func (m *EventMessage) String() string {
    s, err := json.Marshal(*m)
    if err != nil {
        return err.Error()
    }
    return string(s)
}

// EventProcessingError is used to encapsulate the error for the event processing.
type EventProcessingError struct {
    err error
}

func (epe *EventProcessingError) Error() string {
    if epe.err != nil {
        return epe.err.Error()
    }
    return ""
}

// MarshalJSON Amazingly enough, `json` does not know how to Marshal an error; MarshalJSON for the
// EventProcessingError fills the gap,
// so we can serialize an EventErrorMessage with the embedded error.
func (epe EventProcessingError) MarshalJSON() ([]byte, error) {
    return json.Marshal(epe.err.Error())
}

// UnmarshalJSON is the inverse of MarshalJSON and reads in an error description.
// By convention, if the passed in string is `null` this is a no-op.
func (epe EventProcessingError) UnmarshalJSON(data []byte) error {
    s := string(data)
    if s != "null" {
        epe.err = fmt.Errorf(s)
    }
    return nil
}

func NewEventProcessingError(err error) *EventProcessingError {
    return &EventProcessingError{err: err}
}

// An EventErrorMessage encapsulates an error occurred while processing the `Message` and is
// returned over the `notifications` channel to a `Publisher` for eventual upstream processing.
type EventErrorMessage struct {
    Error       EventProcessingError `json:"error,omitempty"`
    ErrorDetail string               `json:"detail,omitempty"` // optional
    Message     *EventMessage        `json:"message,omitempty"`
}

func (m *EventErrorMessage) String() string {
    // FIXME: this probably needs a better approach to omit JSON entirely as apparently
    //  `omitempty` does not work here (and json.Marshal() panics for nil elements).
    if m.Message == nil {
        m.Message = &EventMessage{}
    }
    if m.Error.err == nil {
        m.Error.err = fmt.Errorf("no error")
    }
    s, err := json.Marshal(*m)
    if err != nil {
        return err.Error()
    }
    return string(s)
}

// ErrorMessageWithDetail creates a new EventErrorMessage from the given error and detail (optional,
// can be nil) and an optional EventMessage (can be nil).
// Modeled on the fmt.Error() function.
func ErrorMessageWithDetail(err error, msg *EventMessage, detail string) *EventErrorMessage {
    ret := EventErrorMessage{
        Error:   EventProcessingError{err: err},
        Message: msg,
    }
    if detail != "" {
        ret.ErrorDetail = detail
    }
    return &ret
}

// ErrorMessage creates a new EventErrorMessage from the given error and an EventMessage (can be nil).
func ErrorMessage(err error, msg *EventMessage) *EventErrorMessage {
    return ErrorMessageWithDetail(err, msg, "")
}

// Not really "variables" - but Go is too dumb to figure out they're actually constants.
var (
    // We poll SQS every DefaultPollingInterval seconds
    DefaultPollingInterval, _ = time.ParseDuration("5s")

    // DefaultVisibilityTimeout sets how long SQS will wait for the subscriber to remove the
    // message from the queue.
    // See: https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/sqs-visibility-timeout.html
    DefaultVisibilityTimeout, _ = time.ParseDuration("5s")
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
