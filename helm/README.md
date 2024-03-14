# Helm Chart to Deploy Statemachine Server

*Created by M. Massenzio, 2024*

## Overview

This is a simple Helm chart to deploy the [Statemachine Server](https://github.com/massenz/go-statemachine) on a Kubernetes cluster.

## Install

Simply run:

```shell
helm install -n <ns> fsm ./chart
```


## Test

### AWS Localstack

To test functionality, we use [Localstack](#) deployed as a `Pod` via the `localstack.yaml` spec; once deployed, these steps are necessary to create the SQS queues:

```shell
kubectl exec -it awslocal -- aws configure
kubectl exec -it awslocal -- aws sqs create-queue --region us-west-2 \
     --endpoint-url=http://localhost:4566 --queue-name notifications
kubectl exec -it awslocal -- aws sqs create-queue --region us-west-2 \
     --endpoint-url=http://localhost:4566 --queue-name events
```
*(see the test/mkq.sh script)*

- [ ] This should be implemented as a one-time `Job`.

### Redis

To run a Redis CLI, deploy the `test/redis-cli.yaml` Pod, then run:

```
kubectl exec -it redis-cli -- redis-cli -h redis
```

> Mofidy `redis` with the appropriate namespace if the CLI is running in a different namespace

