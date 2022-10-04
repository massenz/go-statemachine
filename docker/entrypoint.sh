#!/usr/bin/env bash
#
# Copyright (c) 2022 AlertAvert.com.  All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# Author: Marco Massenzio (marco@alertavert.com)
#

set -eu

unset endpoint
if [[ -n ${AWS_ENDPOINT:-} ]]
then
  endpoint="--endpoint-url ${AWS_ENDPOINT}"
fi

cmd="./sm-server -http-port ${SERVER_PORT}  ${endpoint:-} ${DEBUG} \
-redis ${REDIS}:${REDIS_PORT} -timeout ${TIMEOUT:-25ms} -max-retries ${RETRIES:-3} \
-events ${EVENTS_Q} -notifications ${ERRORS_Q} \
$@"

echo $cmd
exec $cmd
