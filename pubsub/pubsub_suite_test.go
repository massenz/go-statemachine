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
	"context"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/massenz/go-statemachine/pubsub"
	"github.com/massenz/statemachine-proto/golang/api"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
)

const (
	localstackImage    = "localstack/localstack:1.3"
	localstackEdgePort = "4566"
	eventsQueue        = "test-events"
	notificationsQueue = "test-notifications"
	acksQueue          = "test-acks"
	timeout            = 1 * time.Second       // Default timeout for Eventually is 1s
	pollingInterval    = 10 * time.Millisecond // Default polling interval for Eventually is 10ms
)

func TestPubSub(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Pub/Sub Suite")
}

type LocalstackContainer struct {
	testcontainers.Container
	EndpointUri string
}

func SetupAwsLocal(ctx context.Context) (*LocalstackContainer, error) {
	req := testcontainers.ContainerRequest{
		Image:        localstackImage,
		ExposedPorts: []string{localstackEdgePort},
		WaitingFor:   wait.ForLog("Ready."),
		Env: map[string]string{
			"AWS_REGION": "us-west-2",
			"EDGE_PORT":  "4566",
			"SERVICES":   "sqs",
		},
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, err
	}

	ip, err := container.Host(ctx)
	if err != nil {
		return nil, err
	}

	mappedPort, err := container.MappedPort(ctx, localstackEdgePort)
	if err != nil {
		return nil, err
	}

	uri := fmt.Sprintf("http://%s:%s", ip, mappedPort.Port())

	return &LocalstackContainer{Container: container, EndpointUri: uri}, nil
}

// Although these are constants, we cannot take the pointers unless we declare them vars.
var (
	region        = "us-west-2"
	awsLocal      *LocalstackContainer
	testSqsClient *sqs.SQS
)

var _ = BeforeSuite(func() {
	Expect(os.Setenv("AWS_REGION", region)).ToNot(HaveOccurred())

	var err error
	awsLocal, err = SetupAwsLocal(context.Background())
	Expect(err).ToNot(HaveOccurred())

	testSqsClient = sqs.New(session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
		Config: aws.Config{
			Endpoint: &awsLocal.EndpointUri,
			Region:   &region,
		},
	})))

	for _, topic := range []string{eventsQueue, notificationsQueue, acksQueue} {
		topic = fmt.Sprintf("%s-%d", topic, GinkgoParallelProcess())

		_, err := testSqsClient.GetQueueUrl(&sqs.GetQueueUrlInput{
			QueueName: &topic,
		})
		if err != nil {
			// the queue does not exist and ought to be created
			_, err = testSqsClient.CreateQueue(&sqs.CreateQueueInput{
				QueueName: &topic,
			})
			Expect(err).NotTo(HaveOccurred())
		}
	}
}, 2.0)

var _ = AfterSuite(func() {
	for _, topic := range []string{eventsQueue, notificationsQueue, acksQueue} {
		topic = getQueueName(topic)

		out, err := testSqsClient.GetQueueUrl(&sqs.GetQueueUrlInput{
			QueueName: &topic,
		})
		Expect(err).NotTo(HaveOccurred())
		if out != nil {
			_, err = testSqsClient.DeleteQueue(&sqs.DeleteQueueInput{QueueUrl: out.QueueUrl})
			Expect(err).NotTo(HaveOccurred())
		}
	}
	awsLocal.Terminate(context.Background())
}, 2.0)

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
	_, err := testSqsClient.SendMessage(&sqs.SendMessageInput{
		MessageBody: aws.String(proto.MarshalTextString(msg)),
		QueueUrl:    &q,
	})
	return err
}
