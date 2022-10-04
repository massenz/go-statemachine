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
    "github.com/golang/protobuf/proto"
    log "github.com/massenz/slf4go/logging"
    protos "github.com/massenz/statemachine-proto/golang/api"
)

// NewSqsPublisher will create a new `Publisher` to send error notifications received on the
// `errorsChannel` to an SQS `dead-letter queue`.
//
// The `awsUrl` is the URL of the AWS SQS service, which can be obtained from the AWS Console,
// or by the local AWS CLI.
func NewSqsPublisher(errorsChannel <-chan protos.EventResponse, awsUrl *string) *SqsPublisher {
    client := getSqsClient(awsUrl)
    if client == nil {
        return nil
    }
    return &SqsPublisher{
        logger: log.NewLog("SQS-Pub"),
        client: client,
        errors: errorsChannel,
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

// Publish sends an error message to the DLQ `topic`
func (s *SqsPublisher) Publish(topic string) {
    queueUrl := GetQueueUrl(s.client, topic)
    s.logger = log.NewLog(fmt.Sprintf("SQS-Pub{%s}", topic))
    s.logger.Info("SQS Publisher started for queue: %s", queueUrl)
    for eventResponse := range s.errors {
        delay := int64(0)
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
