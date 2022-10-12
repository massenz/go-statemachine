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
	"github.com/golang/protobuf/proto"
	log "github.com/massenz/slf4go/logging"
	protos "github.com/massenz/statemachine-proto/golang/api"
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
		logger:        log.NewLog("SQS-Pub"),
		client:        client,
		notifications: channel,
	}
}

// SetLogLevel allows the SqsSubscriber to implement the log.Loggable interface
func (s *SqsPublisher) SetLogLevel(level log.LogLevel) {
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
		log.RootLog.Fatal(fmt.Errorf("cannot get SQS Queue URL for topic %s: %v", topic, err))
	}
	return *out.QueueUrl
}

// Publish sends an message to provided topics depending on SQS Publisher settings.
// If an acksTopic is provided, it will send Ok outcomes to that topic and errors to errorsTopic;
// else, all outcomes will be sent to the errorsTopic. If notifyErrorsOnly is true, only error outcomes
// will be sent.
func (s *SqsPublisher) Publish(errorsTopic string, acksTopic string, notifyErrorsOnly bool) {
	s.logger.Info("SQS Publisher started for topics: %s %s", errorsTopic, acksTopic)
	s.logger.Info("SQS Publisher notifyErrorsOnly: %s", notifyErrorsOnly)

	errorsQueueUrl := GetQueueUrl(s.client, errorsTopic)
	var acksQueueUrl string
	if acksTopic != "" {
		acksQueueUrl = GetQueueUrl(s.client, acksTopic)
	}
	delay := int64(0)
	for eventResponse := range s.notifications {
		isOKOutcome := eventResponse.Outcome != nil && eventResponse.Outcome.Code == protos.EventOutcome_Ok
		if isOKOutcome && notifyErrorsOnly {
			s.logger.Debug("Skipping notification for Ok outcome [Event ID: %s]", eventResponse.EventId)
			continue
		}
		queueUrl := errorsQueueUrl
		if isOKOutcome && acksTopic != "" {
			queueUrl = acksQueueUrl
		}

		s.logger.Debug("[%s] %s", eventResponse.String(), queueUrl)
		msgResult, err := s.client.SendMessage(&sqs.SendMessageInput{
			DelaySeconds: &delay,
			// Encodes the Event as a string, using Protobuf implementation.
			MessageBody: aws.String(proto.MarshalTextString(&eventResponse)),
			QueueUrl:    &queueUrl,
		})
		if err != nil {
			s.logger.Error("Cannot publish eventResponse (%s): %v", eventResponse.String(), err)
			continue
		}
		s.logger.Debug("Notification successfully posted to SQS: %s", *msgResult.MessageId)
	}
	s.logger.Info("SQS Publisher exiting")
}
