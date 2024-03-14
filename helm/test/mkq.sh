#!/usr/bin/env zsh

kubectl exec -it awslocal -- aws configure
kubectl exec -it awslocal -- aws sqs create-queue --region us-west-2 \
     --endpoint-url=http://localhost:4566 --queue-name notifications
kubectl exec -it awslocal -- aws sqs create-queue --region us-west-2 \
     --endpoint-url=http://localhost:4566 --queue-name events
