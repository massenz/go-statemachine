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
	"encoding/base64"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/rs/zerolog/log"
	protos "github.com/massenz/statemachine-proto/golang/api"
	"google.golang.org/protobuf/proto"
)

// ProtoTextMarshaler is an interface that allows for marshaling and unmarshaling of Protobuf messages
// to and from text.
// This is useful when we need to send Protobuf messages as text, for example when using SQS.
type ProtoTextMarshaler interface {
	MarshalToText(proto.Message) (string, error)
	UnmarshalFromText(string, *proto.Message) error
}

// Base64ProtoMarshaler is a simple implementation of the `ProtoTextMarshaler` interface, that
// encodes the Protobuf message as a Base64 string.
type Base64ProtoMarshaler struct{}

func (m *Base64ProtoMarshaler) MarshalToText(msg proto.Message) (string, error) {
	data, err := proto.Marshal(msg)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

func (m *Base64ProtoMarshaler) UnmarshalFromText(text string, msg proto.Message) error {
	data, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		return err
	}
	return proto.Unmarshal(data, msg)
}

// Module-level variable to use as a default implementation of the `ProtoTextMarshaler` interface.
var p = &Base64ProtoMarshaler{}

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
		logger:        log.With().Str("logger", "SQS-Pub").Logger(),
		client:        client,
		notifications: channel,
	}
}

// GetQueueUrl retrieves from AWS SQS the URL for the queue, given the topic name
func GetQueueUrl(client *sqs.SQS, topic string) string {
	out, err := client.GetQueueUrl(&sqs.GetQueueUrlInput{
		QueueName: &topic,
	})
	if err != nil || out.QueueUrl == nil {
		// From the Google School: fail fast and noisily from an unrecoverable error
		log.Fatal().Err(fmt.Errorf("cannot get SQS Queue URL for topic %s", topic)).Msg("cannot get SQS Queue URL")
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
			s.logger.Warn().Msgf("unexpected notification for Ok outcome [Event ID: %s]", eventResponse.EventId)
			continue
		}
		response, err := p.MarshalToText(&eventResponse)
		if err != nil {
			s.logger.Error().Msgf("Cannot marshal eventResponse (%s): %v", eventResponse.String(), err)
			continue
		}
		msgResult, err := s.client.SendMessage(&sqs.SendMessageInput{
			DelaySeconds: &delay,
			// Encodes the Event as a string, using Protobuf implementation.
			MessageBody: aws.String(response),
			QueueUrl:    &errorsQueueUrl,
		})
		if err != nil {
			s.logger.Error().Msgf("Cannot publish eventResponse (%s): %v", eventResponse.String(), err)
			continue
		}
		s.logger.Debug().Msgf("Notification successfully posted to SQS: %s", *msgResult.MessageId)
	}
	s.logger.Info().Msg("SQS publisher exiting")
}
