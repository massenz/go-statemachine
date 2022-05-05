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
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/sqs"
    "github.com/massenz/go-statemachine/logging"
    "strconv"
    "time"
)

// EventMessage abstracts away the details of the actual structure of the events and the actual
// message broker implementation.  It is the Internal Representation (
// IR) for an event being originated by the `sender` and being sent to a `Destination` StateMachine.
type EventMessage struct {
    Sender         string
    Destination    string
    EventId        string
    EventName      string
    EventTimestamp time.Time
}

func (m *EventMessage) String() string {
    return fmt.Sprintf(
        "[%v] %s :: %s :: Dest: %s (From: %s)",
        m.EventTimestamp.String(), m.EventId, m.EventName, m.Destination, m.Sender)
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

// FIXME: need to generalize and abstract the implementation of a Subscriber
type SqsSubscriber struct {
    logger   *logging.Log
    client   *sqs.SQS
    queueUrl *string

    events chan<- EventMessage

    QueueName       *string
    Timeout         time.Duration
    PollingInterval time.Duration
}

// NewSqsSubscriber will create a new `Subscriber` to listen to
// incoming api.Event from a SQS `queue`.
func NewSqsSubscriber(queueName *string, eventsChannel chan<- EventMessage) *SqsSubscriber {
    // TODO: need to confirm this works when running inside an EKS Node.
    sess := session.Must(session.NewSessionWithOptions(session.Options{
        SharedConfigState: session.SharedConfigEnable,
    }))
    client := sqs.New(sess)

    // TODO: Figure out all the error processing
    queueUrl, _ := client.GetQueueUrl(&sqs.GetQueueUrlInput{
        QueueName: queueName,
    })
    return &SqsSubscriber{
        client:   client,
        queueUrl: queueUrl.QueueUrl,
        // TODO: for now just a blocking channel; we will need to confirm
        //  whether we can support a fully concurrent system with a
        //  buffered channel
        events: eventsChannel,
        // Just a signaling channel to communicate to the subscriber method to terminate.
        Timeout:         DefaultVisibilityTimeout,
        PollingInterval: DefaultPollingInterval,
        logger:          logging.NewLog("SQS-Sub"),
    }
}

func (s *SqsSubscriber) SetLogLevel(level logging.LogLevel) {
    s.logger.Level = level
}

// Subscribe runs until signaled on the Done channel and listens for incoming Events
func (s *SqsSubscriber) Subscribe() {
    timeout := int64(s.Timeout.Seconds())
    s.logger.Info("SQS Subscriber started for queue: %s", *s.queueUrl)

    // TODO: do we need to select from done channel to signal we've shut down?
    //  Or this could really run forever until the server is terminated.
    for {
        start := time.Now()
        s.logger.Trace("Polling SQS...")
        msgResult, err := s.client.ReceiveMessage(&sqs.ReceiveMessageInput{
            AttributeNames: []*string{
                aws.String(sqs.MessageSystemAttributeNameSentTimestamp),
            },
            MessageAttributeNames: []*string{
                aws.String(sqs.QueueAttributeNameAll),
            },
            QueueUrl:            s.queueUrl,
            MaxNumberOfMessages: aws.Int64(10),
            VisibilityTimeout:   &timeout,
        })
        if err == nil {
            if len(msgResult.Messages) > 0 {
                s.logger.Debug("Got %d messages", len(msgResult.Messages))
            } else {
                s.logger.Trace("No messages in queue")
            }
            for _, msg := range msgResult.Messages {
                go s.ProcessMessage(msg)
            }
        } else {
            s.logger.Error(err.Error())
        }
        timeLeft := s.PollingInterval - time.Since(start)
        if timeLeft > 0 {
            time.Sleep(timeLeft)
        }
    }
}

func (s *SqsSubscriber) ProcessMessage(msg *sqs.Message) {

    var event = EventMessage{}
    event.Destination = *msg.MessageAttributes["DestinationId"].StringValue
    if event.Destination == "" {
        s.logger.Error("No Destination ID in %v", msg.MessageAttributes)
        // Note: as we return here without deleting the message,
        //if the DLQ is configured after a given number of retries,
        //this message will eventually end up there; however,
        //this is wasteful and no point in retrying something that will forever fail.
        // TODO: find out whether there is a way to post this back directly to DLQ from here.
        return
    }
    event.EventId = *msg.MessageAttributes["EventId"].StringValue
    event.Sender = *msg.MessageAttributes["Sender"].StringValue
    event.EventName = *msg.Body
    timestamp := msg.Attributes[sqs.MessageSystemAttributeNameSentTimestamp]
    if timestamp == nil {
        event.EventTimestamp = time.Now()
    } else {
        // TODO: We may need some amount of error-checking here.
        ts, _ := strconv.ParseInt(*timestamp, 10, 64)
        event.EventTimestamp = time.UnixMilli(ts)
    }
    s.logger.Debug("Sent at: %s", event.EventTimestamp.String())

    s.events <- event
    s.logger.Debug("Removing message %v from SQS", *msg.MessageId)
    _, err := s.client.DeleteMessage(&sqs.DeleteMessageInput{
        QueueUrl:      s.queueUrl,
        ReceiptHandle: msg.ReceiptHandle,
    })
    if err != nil {
        s.logger.Error("failed to remove message %v from SQS: %v", msg.MessageId, err)
    }
}
