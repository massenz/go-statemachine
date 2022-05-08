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
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/sqs"
    "github.com/massenz/go-statemachine/logging"
    "strconv"
    "time"
)

// FIXME: need to generalize and abstract the implementation of a Subscriber
type SqsSubscriber struct {
    logger          *logging.Log
    client          *sqs.SQS
    events          chan<- EventMessage
    Timeout         time.Duration
    PollingInterval time.Duration
}

// NewSqsSubscriber will create a new `Subscriber` to listen to
// incoming api.Event from a SQS `queue`.
func NewSqsSubscriber(eventsChannel chan<- EventMessage) *SqsSubscriber {
    // TODO: need to confirm this works when running inside an EKS Node.
    sess := session.Must(session.NewSessionWithOptions(session.Options{
        SharedConfigState: session.SharedConfigEnable,
    }))
    client := sqs.New(sess)
    return &SqsSubscriber{
        logger:          logging.NewLog("SQS-Sub"),
        client:          client,
        events:          eventsChannel,
        Timeout:         DefaultVisibilityTimeout,
        PollingInterval: DefaultPollingInterval,
    }
}

// SetLogLevel allows the SqsSubscriber to implement the logging.Loggable interface
func (s *SqsSubscriber) SetLogLevel(level logging.LogLevel) {
    s.logger.Level = level
}

// GetQueueUrlFromTopic retrieves from AWS SQS the URL for the queue, given the topic name
func (s *SqsSubscriber) GetQueueUrlFromTopic(topic string) (*string, error) {
    out, err := s.client.GetQueueUrl(&sqs.GetQueueUrlInput{
        QueueName: &topic,
    })
    if err != nil {
        s.logger.Error("Cannot obtain URL for SQS Topic %s: %v", topic, err)
        return nil, err
    }
    return out.QueueUrl, nil
}

// Subscribe runs until signaled on the Done channel and listens for incoming Events
func (s *SqsSubscriber) Subscribe(topic string) {
    queueUrl, err := s.GetQueueUrlFromTopic(topic)
    if err != nil {
        // From the Google School: fail fast and noisily from an unrecoverable error
        panic(err)
    }
    s.logger.Info("SQS Subscriber started for queue: %s", *queueUrl)

    timeout := int64(s.Timeout.Seconds())
    // This will run forever until the server is terminated.
    for {
        start := time.Now()
        s.logger.Trace("Polling SQS at %v", start)
        msgResult, err := s.client.ReceiveMessage(&sqs.ReceiveMessageInput{
            AttributeNames: []*string{
                aws.String(sqs.MessageSystemAttributeNameSentTimestamp),
            },
            MessageAttributeNames: []*string{
                aws.String(sqs.QueueAttributeNameAll),
            },
            QueueUrl:            queueUrl,
            MaxNumberOfMessages: aws.Int64(10),
            VisibilityTimeout:   &timeout,
        })
        if err == nil {
            if len(msgResult.Messages) > 0 {
                s.logger.Debug("Got %d messages", len(msgResult.Messages))
            } else {
                s.logger.Trace("no messages in queue")
            }
            for _, msg := range msgResult.Messages {
                s.logger.Trace("processing %v", msg.String())
                go s.ProcessMessage(msg, queueUrl)
            }
        } else {
            s.logger.Error(err.Error())
        }
        timeLeft := s.PollingInterval - time.Since(start)
        if timeLeft > 0 {
            s.logger.Trace("sleeping for %v", timeLeft)
            time.Sleep(timeLeft)
        }
    }
}

func (s *SqsSubscriber) ProcessMessage(msg *sqs.Message, queueUrl *string) {

    var event = EventMessage{}
    event.Destination = *msg.MessageAttributes["DestinationId"].StringValue
    event.EventName = *msg.Body

    if event.Destination != "" && event.EventName != "" {
        event.EventId = *msg.MessageAttributes["EventId"].StringValue
        event.Sender = *msg.MessageAttributes["Sender"].StringValue
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
    } else {
        // TODO: publish error to DLQ, no point in retrying.
        s.logger.Error("No Destination ID or Event in %v", msg.String())
    }

    s.logger.Debug("Removing message %v from SQS", *msg.MessageId)
    _, err := s.client.DeleteMessage(&sqs.DeleteMessageInput{
        QueueUrl:      queueUrl,
        ReceiptHandle: msg.ReceiptHandle,
    })
    if err != nil {
        // TODO: publish error to DLQ, we should probably also alert.
        s.logger.Error("Failed to remove message %v from SQS: %v", msg.MessageId, err)
    }
    s.logger.Trace("Message removed")
}
