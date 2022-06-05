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
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/massenz/go-statemachine/logging"
)

type SqsPublisher struct {
	logger *logging.Log
	client *sqs.SQS
	errors <-chan EventErrorMessage
}

// NewSqsPublisher will create a new `Publisher` to send error notifications to
// an SQS `dead-letter queue`.
func NewSqsPublisher(errorsChannel <-chan EventErrorMessage, sqsUrl *string) *SqsPublisher {
	client := getSqsClient(sqsUrl)
	if client == nil {
		return nil
	}
	return &SqsPublisher{
		logger: logging.NewLog("SQS-Pub"),
		client: client,
		errors: errorsChannel,
	}
}

// SetLogLevel allows the SqsSubscriber to implement the logging.Loggable interface
func (s *SqsPublisher) SetLogLevel(level logging.LogLevel) {
	if s == nil {
		fmt.Println("WARN: attempting to set log level on nil Publisher")
		return
	}
	s.logger.Level = level
}

// GetQueueUrl retrieves from AWS SQS the URL for the queue, given the topic name
func GetQueueUrl(client *sqs.SQS, topic string) string {
	out, err := client.GetQueueUrl(&sqs.GetQueueUrlInput{
		QueueName: &topic,
	})
	if err != nil || out.QueueUrl == nil {
		// From the Google School: fail fast and noisily from an unrecoverable error
		logging.RootLog.Fatal(fmt.Errorf("cannot get SQS Queue URL for topic %s: %v", topic, err))
	}
	return *out.QueueUrl
}

// Publish sends an error message to the DLQ `topic`
func (s *SqsPublisher) Publish(topic string) {
	queueUrl := GetQueueUrl(s.client, topic)
	s.logger.Info("SQS Publisher started for queue: %s", queueUrl)

	for msg := range s.errors {
		delay := int64(0)
		s.logger.Debug("Publishing %s to %s", msg.String(), queueUrl)
		msgResult, err := s.client.SendMessage(&sqs.SendMessageInput{
			DelaySeconds:      &delay,
			MessageAttributes: makeAttributes(msg),
			MessageBody:       aws.String(msg.Message.String()),
			QueueUrl:          &queueUrl,
		})
		if err != nil {
			s.logger.Error("Cannot publish msg (%s): %v", msg.String(), err)
			continue
		}
		s.logger.Debug("Error msg sent to DLQ: %s", *msgResult.MessageId)
	}
	s.logger.Info("SQS Publisher exiting")
}

// makeAttributes is necessary as we can't just copy the strings into the MessageAttributeValue
// values, as empty strings will cause an error with SQS.
// This function will only add those keys for which we have an actual value.
func makeAttributes(msg EventErrorMessage) map[string]*sqs.MessageAttributeValue {
	res := make(map[string]*sqs.MessageAttributeValue)
	if msg.Error.Error() != "" {
		res["Error"] = &sqs.MessageAttributeValue{
			DataType:    aws.String("String"),
			StringValue: aws.String(msg.Error.Error()),
		}
	}
	if msg.Message != nil && msg.Message.EventId != "" {
		res["EventId"] = &sqs.MessageAttributeValue{
			DataType:    aws.String("String"),
			StringValue: aws.String(msg.Message.EventId),
		}
	}
	if msg.ErrorDetail != "" {
		res["ErrorDetails"] = &sqs.MessageAttributeValue{
			DataType:    aws.String("String"),
			StringValue: aws.String(msg.ErrorDetail),
		}
	}
	return res
}
