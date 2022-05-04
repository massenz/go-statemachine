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
    "github.com/golang/protobuf/proto"
    "github.com/massenz/go-statemachine/api"
    "github.com/massenz/go-statemachine/logging"
    "github.com/massenz/go-statemachine/storage"
    "time"
)

// FIXME: need to generalize and abstract the implementation.
// TODO: this is just a first pass to explore possible alternatives.

// Not really "variables" - but Go is too dumb to figure out they're actually constants.
var (
    // We poll SQS every DefaultPollingInterval seconds
    DefaultPollingInterval, _ = time.ParseDuration("5s")

    // DefaultVisibilityTimeout sets how long SQS will wait for the subscriber to remove the
    // message from the queue.
    // See: https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/sqs-visibility-timeout.html
    DefaultVisibilityTimeout, _ = time.ParseDuration("5s")
)

type SqsSubscriber struct {
    client   *sqs.SQS
    queueUrl *string
    // TODO: add an Events channel
    // TODO: add a Done channel

    QueueName       *string
    Timeout         time.Duration
    PollingInterval time.Duration
    StoreManager    storage.StoreManager
    Logger          *logging.Log
}

// SqsSubscriber implements the logging.Loggable interface
func (s *SqsSubscriber) SetLogLevel(level logging.LogLevel) {
    s.Logger.Level = level
}

// NewSqsSubscriber will create a new `Subscriber` to listen to
// incoming api.Event from a SQS `queue`.
func NewSqsSubscriber(queueName *string, store storage.StoreManager) *SqsSubscriber {
    // TODO: allow to specify a --profile (currently using AWS_PROFILE by default)
    // Specify profile to load for the session's config
    //      sess, err := session.NewSessionWithOptions(session.Options{
    //          Profile: "profile_name",
    //      })

    sess := session.Must(session.NewSessionWithOptions(session.Options{
        SharedConfigState: session.SharedConfigEnable,
    }))
    client := sqs.New(sess)

    // TODO: Figure out all the error processing
    queueUrl, _ := client.GetQueueUrl(&sqs.GetQueueUrlInput{
        QueueName: queueName,
    })

    return &SqsSubscriber{
        client:          client,
        queueUrl:        queueUrl.QueueUrl,
        Timeout:         DefaultVisibilityTimeout,
        PollingInterval: DefaultPollingInterval,
        Logger:          logging.NewLog("SQS-Sub"),
        StoreManager:    store,
    }
}

// Subscribe runs until signaled on the Done channel and listens for incoming Events
func (s *SqsSubscriber) Subscribe() {
    timeout := int64(s.Timeout.Seconds())

    for {
        start := time.Now()
        s.Logger.Trace("Polling SQS...")
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
                s.Logger.Debug("Got %d messages", len(msgResult.Messages))
            } else {
                s.Logger.Trace("No messages in queue")
            }
            for _, msg := range msgResult.Messages {
                go s.ProcessMessage(msg)
                s.Logger.Debug("Removing message %v from SQS", *msg.MessageId)
                s.client.DeleteMessage(&sqs.DeleteMessageInput{
                    QueueUrl:      s.queueUrl,
                    ReceiptHandle: msg.ReceiptHandle,
                })
            }
        } else {
            s.Logger.Error(err.Error())
        }
        timeLeft := s.PollingInterval - time.Since(start)
        if timeLeft > 0 {
            time.Sleep(timeLeft)
        }
    }
}

func (s *SqsSubscriber) ProcessMessage(msg *sqs.Message) {

    var evt api.Event
    if err := proto.UnmarshalText(*msg.Body, &evt); err != nil {
        s.Logger.Error(err.Error())
        return
    }
    stateMachineId := msg.MessageAttributes["DestinationId"].StringValue
    if stateMachineId == nil {
        s.Logger.Error("No Destination ID in %v", msg.MessageAttributes)
        return
    }

    // TODO: send the ID, Evt on the Events channel here

    // TODO: move the code below to an EventsListener
    fsm, ok := s.StoreManager.GetStateMachine(*stateMachineId)
    if !ok {
        s.Logger.Error("StateMachine [%s] could not be found", *stateMachineId)
        return
    }
    // TODO: cache the configuration locally: they are immutable anyway.
    cfg, ok := s.StoreManager.GetConfig(fsm.ConfigId)
    if !ok {
        s.Logger.Error("Configuration [%s] could not be found", fsm.ConfigId)
        return
    }

    cfgFsm := api.ConfiguredStateMachine{
        Config: cfg,
        FSM:    fsm,
    }
    if err := cfgFsm.SendEvent(evt.Transition.Event); err != nil {
        s.Logger.Error("Cannot send event to FSM %s: %s", *stateMachineId, err.Error())
        return
    }
    err := s.StoreManager.PutStateMachine(*stateMachineId, fsm)
    if err != nil {
        s.Logger.Error(err.Error())
        return
    }
    s.Logger.Debug("Event %s for FSM %s processed", evt, fsm)
}
