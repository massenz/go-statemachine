#!/usr/bin/env bash
#
# Copyright (c) 2022 AlertAvert.com.  All rights reserved.
#
# Licensed under the Apache License, Version 2.0
# http://www.apache.org/licenses/LICENSE-2.0
#
# Author: Marco Massenzio (marco@alertavert.com)
#

set -eu

declare endpoint=""
declare acks=""
declare errors_only=""
declare notifications=""
declare retries=""
declare timeout=""

if [[ -n ${AWS_ENDPOINT:-} ]]
then
  endpoint="-endpoint-url ${AWS_ENDPOINT}"
fi

if [[ -n ${ERRORS_ONLY:-} ]]
then
  errors_only="-notify-errors-only"
fi

if [[ -n ${ACKS_Q:-} ]]
then
  acks="-acks ${ACKS_Q}"
fi

if [[ -n ${NOTIFICATIONS_Q:-} ]]
then
  notifications="-notifications ${NOTIFICATIONS_Q}"
fi

if [[ -n ${TIMEOUT:-} ]]
then
  timeout="-timeout ${TIMEOUT}"
fi

if [[ -n ${RETRIES:-} ]]
then
  retries="-max-retries ${RETRIES}"
fi

cmd="./sm-server -http-port ${SERVER_PORT} -grpc-port ${GRPC_PORT} \
-events ${EVENTS_Q} -redis ${REDIS}:${REDIS_PORT} \
${CLUSTER:-} ${DEBUG:-} ${endpoint} \
${errors_only} ${timeout} ${retries} \
${acks} ${notifications} \
$@"

echo $cmd
exec $cmd
