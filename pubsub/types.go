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
    "github.com/massenz/go-statemachine/storage"
    log "github.com/massenz/slf4go/logging"
    protos "github.com/massenz/statemachine-proto/golang/api"
    "time"
)

// EventProcessingError is used to encapsulate the error for the event processing.
type EventProcessingError struct {
    err error
}

func (epe EventProcessingError) Error() string {
    if epe.err != nil {
        return epe.err.Error()
    }
    return ""
}

// MarshalJSON Amazingly enough, `json` does not know how to Marshal an error; MarshalJSON for the
// EventProcessingError fills the gap,
// so we can serialize an EventErrorMessage with the embedded error.
func (epe EventProcessingError) MarshalJSON() ([]byte, error) {
    return json.Marshal(epe.Error())
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

// FIXME: this will soon be replaced instead by a Protobuf message.
// An EventErrorMessage encapsulates an error occurred while processing the `Message` and is
// returned over the `notifications` channel to a `Publisher` for eventual upstream processing.
type EventErrorMessage struct {
    Error       EventProcessingError `json:"error,omitempty"`
    ErrorDetail string               `json:"detail,omitempty"` // optional
    Message     *protos.Event        `json:"message,omitempty"`
}

func (m *EventErrorMessage) String() string {
    s, err := json.Marshal(*m)
    if err != nil {
        return err.Error()
    }
    return string(s)
}

// ErrorMessage creates a new EventErrorMessage from the given error and detail (optional,
// can be nil) and an optional EventMessage (can be nil).
// Modeled on the fmt.Error() function.
func ErrorMessage(err error, msg *protos.Event, detail string) *EventErrorMessage {
    return &EventErrorMessage{
        Error:       EventProcessingError{err: err},
        Message:     msg,
        ErrorDetail: detail,
    }
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
    logger        *log.Log
    events        <-chan protos.EventRequest
    notifications chan<- EventErrorMessage
    store         storage.StoreManager
}

// ListenerOptions are used to configure an EventsListener at creation and are used
// to decouple the internals of the listener from its exposed configuration.
type ListenerOptions struct {
    EventsChannel        <-chan protos.EventRequest
    NotificationsChannel chan<- EventErrorMessage
    StatemachinesStore   storage.StoreManager
    ListenersPoolSize    int8
}
