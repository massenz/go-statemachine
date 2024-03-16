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
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/massenz/go-statemachine/pkg/api"
	log "github.com/massenz/slf4go/logging"
	"google.golang.org/protobuf/proto"
	"os"
	"time"

	protos "github.com/massenz/statemachine-proto/golang/api"
)

// getSqsClient connects to AWS and obtains an SQS client; passing `nil` as the `awsEndpointUrl` will
// connect by default to AWS; use a different (possibly local) URL for a LocalStack test deployment.
func getSqsClient(awsEndpointUrl *string) *sqs.SQS {
	var sess *session.Session
	if awsEndpointUrl == nil {
		sess = session.Must(session.NewSessionWithOptions(session.Options{
			SharedConfigState: session.SharedConfigEnable,
		}))
	} else {
		region, found := os.LookupEnv("AWS_REGION")
		if !found {
			fmt.Printf("No AWS Region configured, cannot connect to SQS provider at %s\n",
				*awsEndpointUrl)
			return nil
		}
		sess = session.Must(session.NewSessionWithOptions(session.Options{
			SharedConfigState: session.SharedConfigEnable,
			Config: aws.Config{
				Endpoint: awsEndpointUrl,
				Region:   &region,
			},
		}))
	}
	return sqs.New(sess)
}

// NewSqsSubscriber will create a new `Subscriber` to listen to
// incoming api.Event from a SQS `queue`.
func NewSqsSubscriber(eventsChannel chan<- protos.EventRequest, sqsUrl *string) *SqsSubscriber {
	client := getSqsClient(sqsUrl)
	if client == nil {
		return nil
	}
	return &SqsSubscriber{
		logger:               log.NewLog("SQS-Sub"),
		client:               client,
		events:               eventsChannel,
		Timeout:              DefaultVisibilityTimeout,
		PollingInterval:      DefaultPollingInterval,
		MessageRemoveRetries: DefaultRetries,
	}
}

// SetLogLevel allows the SqsSubscriber to implement the log.Loggable interface
func (s *SqsSubscriber) SetLogLevel(level log.LogLevel) {
	if s == nil || s.logger == nil {
		fmt.Println("WARN: attempt to set Log level on a nil SqsSubscriber")
		return
	}
	s.logger.Level = level
}

// Subscribe runs until signaled on the Done channel and listens for incoming Events
func (s *SqsSubscriber) Subscribe(topic string, done <-chan interface{}) {
	queueUrl := GetQueueUrl(s.client, topic)
	s.logger.Name = fmt.Sprintf("%s{%s}", s.logger.Name, topic)
	s.logger.Info("SQS Subscriber started for queue: %s", queueUrl)

	timeout := int64(s.Timeout.Seconds())
	for {
		select {
		case <-done:
			s.logger.Info("SQS Subscriber terminating")
			return
		default:
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

	// The body of the message (the actual request) is mandatory.
	if msg.Body == nil {
		s.logger.Error("Message %v has no body", msg.MessageId)
		// TODO: publish error to DLQ.
		return
	}
	var request protos.EventRequest
	err := proto.Unmarshal([]byte(*msg.Body), &request)
	if err != nil {
		s.logger.Error("message %v has invalid body: %s", msg.MessageId, err.Error())
		// TODO: publish error to DLQ.
		return
	}

	destId := request.GetId()
	if destId == "" {
		errDetails := fmt.Sprintf("no Destination ID in %v", request.String())
		s.logger.Error(errDetails)
		// TODO: publish error to DLQ.
		return
	}
	// The Event ID and timestamp are optional and, if missing, will be generated here.
	api.UpdateEvent(request.Event)
	s.events <- request

	for i := 0; i < s.MessageRemoveRetries; i++ {
		s.logger.Debug("removing message %v from SQS", *msg.MessageId)
		_, err = s.client.DeleteMessage(&sqs.DeleteMessageInput{
			QueueUrl:      queueUrl,
			ReceiptHandle: msg.ReceiptHandle,
		})
		if err != nil {
			errDetails := fmt.Sprintf("failed to remove message %v from SQS (attempt: %d)",
				msg.MessageId, i+1)
			s.logger.Error("%s: %v", errDetails, err)
		} else {
			break
		}
	}
	s.logger.Trace("message %v removed", msg.MessageId)
}
