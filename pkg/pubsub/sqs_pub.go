/*
 * Copyright (c) 2022 AlertAvert.com.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Author: Marco Massenzio (marco@alertavert.com)
 */

package pubsub

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sqs"
	slf4go "github.com/massenz/slf4go/logging"
	protos "github.com/massenz/statemachine-proto/golang/api"
	"google.golang.org/protobuf/proto"
)

// NewSqsPublisher will create a new `Publisher` to send error notifications received on the
// `errorsChannel` to an SQS `dead-letter queue`.
//
// The `awsUrl` is the URL of the AWS SQS service, which can be obtained from the AWS Console,
// or by the local AWS CLI.
func NewSqsPublisher(channel <-chan protos.EventResponse, awsUrl *string) *SqsPublisher {
	client := getSqsClient(awsUrl)
	if client == nil {
		return nil
	}
	return &SqsPublisher{
		logger:        slf4go.NewLog("SQS-Pub"),
		client:        client,
		notifications: channel,
	}
}

// SetLogLevel allows the SqsPublisher to implement the log.Loggable interface
func (s *SqsPublisher) SetLogLevel(level slf4go.LogLevel) {
	if s == nil || s.logger == nil {
		fmt.Println("WARN: attempt to set Log level on a nil SqsPublisher")
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
		slf4go.RootLog.Fatal(fmt.Errorf("cannot get SQS Queue URL for topic %s: %v", topic, err))
	}
	return *out.QueueUrl
}

// Publish receives notifications from the SqsPublisher channel, and sends a message to a topic.
func (s *SqsPublisher) Publish(errorsTopic string) {
	errorsQueueUrl := GetQueueUrl(s.client, errorsTopic)
	delay := int64(0)
	for eventResponse := range s.notifications {
		isOKOutcome := eventResponse.Outcome != nil && eventResponse.Outcome.Code == protos.EventOutcome_Ok
		if isOKOutcome {
			s.logger.Warn("unexpected notification for Ok outcome [Event ID: %s]", eventResponse.EventId)
			continue
		}
		response, err := proto.Marshal(&eventResponse)
		if err != nil {
			s.logger.Error("Cannot marshal eventResponse (%s): %v", eventResponse.String(), err)
			continue
		}
		msgResult, err := s.client.SendMessage(&sqs.SendMessageInput{
			DelaySeconds: &delay,
			// Encodes the Event as a string, using Protobuf implementation.
			MessageBody: aws.String(string(response)),
			QueueUrl:    &errorsQueueUrl,
		})
		if err != nil {
			s.logger.Error("Cannot publish eventResponse (%s): %v", eventResponse.String(), err)
			continue
		}
		s.logger.Debug("Notification successfully posted to SQS: %s", *msgResult.MessageId)
	}
	s.logger.Info("SQS Publisher exiting")
}
