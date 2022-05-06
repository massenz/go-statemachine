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
    "time"
)

// EventMessage abstracts away the details of the actual structure of the events and the actual
// message broker implementation.  It is the Internal Representation (
// IR) for an event being originated by the `sender` and being sent to a `Destination` StateMachine.
type EventMessage struct {
    Sender         string    `json:"sender"`
    Destination    string    `json:"destination"`
    EventId        string    `json:"event_id"`
    EventName      string    `json:"event_name"`
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

// MarshalJSON Amazingly enough, `json` does not know how to Marshal an error; MarshalJSON for the
// EventProcessingError fills the gap,
// so we can serialize an EventErrorMessage with the embedded error.
func (epe EventProcessingError) MarshalJSON() ([]byte, error) {
    return json.Marshal(epe.err.Error())
}

// UnmarshalJSON is the opposite of MarshalJSON and reads in an error description.
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
    Error       EventProcessingError `json:"error"`
    ErrorDetail string               `json:"detail"` // optional
    Message     *EventMessage        `json:"message"`
}

func (m *EventErrorMessage) String() string {
    s, err := json.Marshal(*m)
    if err != nil {
        return err.Error()
    }
    return string(s)
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
