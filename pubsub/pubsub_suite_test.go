/*
 * Copyright (c) 2022 AlertAvert.com.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Author: Marco Massenzio (marco@alertavert.com)
 */

package pubsub_test

import (
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/massenz/go-statemachine/pubsub"
	"github.com/massenz/statemachine-proto/golang/api"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"

	log "github.com/massenz/slf4go/logging"
)

const (
	timeout            = 5 * time.Second
	channelWait        = 10 * time.Millisecond
	eventsQueue        = "test-events"
	notificationsQueue = "test-notifications"
	acksQueue          = "test-acks"
)

func TestPubSub(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Pub/Sub Suite")
}

// Although these are constants, we cannot take the pointers unless we declare them vars.
var (
	sqsUrl        = "http://localhost:4566"
	region        = "us-west-2"
	testSqsClient = sqs.New(session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
		Config: aws.Config{
			Endpoint: &sqsUrl,
			Region:   &region,
		},
	})))
	testLog = log.NewLog("PUBSUB")
)

var _ = BeforeSuite(func() {
	testLog.Level = log.NONE
	Expect(os.Setenv("AWS_REGION", region)).ToNot(HaveOccurred())
	for _, topic := range []string{eventsQueue, notificationsQueue, acksQueue} {
		topic = fmt.Sprintf("%s-%d", topic, GinkgoParallelProcess())

		_, err := testSqsClient.GetQueueUrl(&sqs.GetQueueUrlInput{
			QueueName: &topic,
		})
		if err != nil {
			// the queue does not exist and ought to be created
			testLog.Info("Creating SQS Queue %s", topic)
			_, err = testSqsClient.CreateQueue(&sqs.CreateQueueInput{
				QueueName: &topic,
			})
			Expect(err).NotTo(HaveOccurred())
		}
	}
})

var _ = AfterSuite(func() {
	for _, topic := range []string{eventsQueue, notificationsQueue, acksQueue} {
		topic = getQueueName(topic)

		out, err := testSqsClient.GetQueueUrl(&sqs.GetQueueUrlInput{
			QueueName: &topic,
		})
		Expect(err).NotTo(HaveOccurred())
		if out != nil {
			testLog.Info("Deleting SQS Queue %s", topic)
			_, err = testSqsClient.DeleteQueue(&sqs.DeleteQueueInput{QueueUrl: out.QueueUrl})
			Expect(err).NotTo(HaveOccurred())
		}
	}
})

// getQueueName provides a way to obtain a process-independent name for the SQS queue,
// when Ginkgo tests are run in parallel (-p)
func getQueueName(topic string) string {
	return fmt.Sprintf("%s-%d", topic, GinkgoParallelProcess())
}

func getSqsMessage(queue string) *sqs.Message {
	q := pubsub.GetQueueUrl(testSqsClient, queue)
	out, err := testSqsClient.ReceiveMessage(&sqs.ReceiveMessageInput{
		QueueUrl: &q,
	})
	Expect(err).ToNot(HaveOccurred())
	// It could be that the message is not yet available, so we need to retry
	if len(out.Messages) == 0 {
		return nil
	}
	Expect(len(out.Messages)).To(Equal(1))
	_, err = testSqsClient.DeleteMessage(&sqs.DeleteMessageInput{
		QueueUrl:      &q,
		ReceiptHandle: out.Messages[0].ReceiptHandle,
	})
	Expect(err).ToNot(HaveOccurred())
	return out.Messages[0]
}

// postSqsMessage mirrors the decoding of the SQS Message in the Subscriber and will
// send it over the `queue`, so that we can test the Publisher can correctly receive it.
func postSqsMessage(queue string, msg *api.EventRequest) error {
	q := pubsub.GetQueueUrl(testSqsClient, queue)
	testLog.Debug("Post Message -- Timestamp: %v", msg.Event.Timestamp)
	_, err := testSqsClient.SendMessage(&sqs.SendMessageInput{
		MessageBody: aws.String(proto.MarshalTextString(msg)),
		QueueUrl:    &q,
	})
	return err
}
