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
	log "github.com/massenz/slf4go/logging"
	"os"
	"strconv"
	"time"
)

// TODO: should we need to generalize and abstract the implementation of a Subscriber?
//  This would be necessary if we were to implement a different message broker (e.g., Kafka)

type SqsSubscriber struct {
	logger          *log.Log
	client          *sqs.SQS
	events          chan<- EventMessage
	Timeout         time.Duration
	PollingInterval time.Duration
}

// getSqsClient connects to AWS and obtains an SQS client; passing `nil` as the `sqsUrl` will
// connect by default to AWS; use a different (possibly local) URL for a LocalStack test deployment.
func getSqsClient(sqsUrl *string) *sqs.SQS {
	var sess *session.Session
	if sqsUrl == nil {
		sess = session.Must(session.NewSessionWithOptions(session.Options{
			SharedConfigState: session.SharedConfigEnable,
		}))
	} else {
		region, found := os.LookupEnv("AWS_REGION")
		if !found {
			fmt.Printf("No AWS Region configured, cannot connect to SQS provider at %s\n",
				*sqsUrl)
			return nil
		}
		sess = session.Must(session.NewSessionWithOptions(session.Options{
			SharedConfigState: session.SharedConfigEnable,
			Config: aws.Config{
				Endpoint: sqsUrl,
				Region:   &region,
			},
		}))
	}
	return sqs.New(sess)
}

// NewSqsSubscriber will create a new `Subscriber` to listen to
// incoming api.Event from a SQS `queue`.
func NewSqsSubscriber(eventsChannel chan<- EventMessage, sqsUrl *string) *SqsSubscriber {
	client := getSqsClient(sqsUrl)
	if client == nil {
		return nil
	}
	return &SqsSubscriber{
		logger:          log.NewLog("SQS-Sub"),
		client:          client,
		events:          eventsChannel,
		Timeout:         DefaultVisibilityTimeout,
		PollingInterval: DefaultPollingInterval,
	}
}

// SetLogLevel allows the SqsSubscriber to implement the log.Loggable interface
func (s *SqsSubscriber) SetLogLevel(level log.LogLevel) {
	s.logger.Level = level
}

// Subscribe runs until signaled on the Done channel and listens for incoming Events
func (s *SqsSubscriber) Subscribe(topic string, done <-chan interface{}) {
	queueUrl := GetQueueUrl(s.client, topic)
	s.logger.Info("SQS Subscriber started for queue: %s", queueUrl)

	timeout := int64(s.Timeout.Seconds())
	for {
		select {
		case <-done:
			s.logger.Info("SQS Subscriber terminating")
			return
		default:
			s.logger.Trace("...")
		}
		start := time.Now()
		s.logger.Trace("Polling SQS at %v", start)
		msgResult, err := s.client.ReceiveMessage(&sqs.ReceiveMessageInput{
			AttributeNames: []*string{
				aws.String(sqs.MessageSystemAttributeNameSentTimestamp),
			},
			MessageAttributeNames: []*string{
				aws.String(sqs.QueueAttributeNameAll),
			},
			QueueUrl:            &queueUrl,
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
				go s.ProcessMessage(msg, &queueUrl)
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
	s.logger.Trace("Processing Message %v", msg.MessageId)
	var event = EventMessage{}
	event.Destination = *msg.MessageAttributes["DestinationId"].StringValue
	event.EventName = *msg.Body

	if event.Destination != "" && event.EventName != "" {
		event.EventId = *msg.MessageAttributes["EventId"].StringValue
		event.Sender = *msg.MessageAttributes["Sender"].StringValue
		timestamp := msg.Attributes[sqs.MessageSystemAttributeNameSentTimestamp]
		if timestamp == nil {
			s.logger.Warn("No Timestamp in received event, using current time")
			event.EventTimestamp = time.Now()
		} else {
			// An SQS Message timestamp is a Unix milliseconds from epoch.
			// TODO: We may need some amount of error-checking here.
			ts, _ := strconv.ParseInt(*timestamp, 10, 64)
			event.EventTimestamp = time.UnixMilli(ts)
		}
		s.logger.Debug("Sent at: %s", event.EventTimestamp.String())
		s.events <- event
	} else {
		errDetails := fmt.Sprintf("No Destination ID or Event in %v", msg.String())
		s.logger.Error(errDetails)
		ErrorMessage(fmt.Errorf(errDetails), &event)
		// TODO: publish error to DLQ.
	}

	s.logger.Debug("Removing message %v from SQS", *msg.MessageId)
	_, err := s.client.DeleteMessage(&sqs.DeleteMessageInput{
		QueueUrl:      queueUrl,
		ReceiptHandle: msg.ReceiptHandle,
	})
	if err != nil {
		errDetails := fmt.Sprintf("Failed to remove message %v from SQS", msg.MessageId)
		s.logger.Error("%s: %v", errDetails, err)
		ErrorMessageWithDetail(err, &event, errDetails)
		// TODO: publish error to DLQ, should also retry removal here.
	}
	s.logger.Trace("Message %v removed", msg.MessageId)
}
