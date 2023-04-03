#!/usr/bin/env bash
#
# Copyright (c) 2022-2023 AlertAvert.com.  All rights reserved.
#
# Licensed under the Apache License, Version 2.0
# http://www.apache.org/licenses/LICENSE-2.0
#
# Author: Marco Massenzio (marco@alertavert.com)

set -eu

declare endpoint=""
declare events=""
declare notifications=""
declare retries=""
declare timeout=""

if [[ -n ${AWS_ENDPOINT:-} ]]
then
  endpoint="-endpoint-url ${AWS_ENDPOINT}"
fi

if [[ -n ${EVENTS_Q:-} ]]
then
  events="-events ${EVENTS_Q}"
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

pwd
ls -lA

cmd="./fsm-server -grpc-port ${GRPC_PORT} -redis ${REDIS:-} ${DEBUG:-} ${INSECURE:-}
${endpoint} ${timeout} ${retries} ${events} ${notifications} $@"

echo $cmd
exec $cmd
