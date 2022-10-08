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

unset endpoint
if [[ -n ${AWS_ENDPOINT:-} ]]
then
  endpoint="--endpoint-url ${AWS_ENDPOINT}"
fi

# Support for optional outcomes queue
OUTCOMES=$([ -n "$OUTCOMES_Q" ] && echo "-outcomes $OUTCOMES_Q" || echo "")

cmd="./sm-server -http-port ${SERVER_PORT}  ${endpoint:-} ${CLUSTER} ${DEBUG} \
-redis ${REDIS}:${REDIS_PORT} -timeout ${TIMEOUT:-25ms} -max-retries ${RETRIES:-3} \
-events ${EVENTS_Q} -notifications ${ERRORS_Q} $OUTCOMES \
$@"

echo $cmd
exec $cmd
