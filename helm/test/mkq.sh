#!/usr/bin/env zsh
#
# Create queues in localstack
# Usage: ./mkq.sh
#
# This script creates queues in localstack using the AWS CLI.
set -eu

declare -a queues=("notifications" "events")
declare region=${AWS_REGION:-"us-west-2"}
declare endpoint="http://localhost:4566"

kubectl exec -it awslocal -- aws configure
for queue in "${queues[@]}"; do
  kubectl exec -it awslocal -- aws sqs create-queue --region $region \
    --endpoint-url=$endpoint --queue-name $queue
done
