#!/usr/bin/env zsh

# This will fail if multiple versions of the server
# binary are present in the out/bin directory.
alias fsm=$(ls out/bin/fsm-server-*)
alias fsmctl=$(ls out/bin/fsm-cli-*)

for port in $(seq 7494 7498)
do
  fsm -redis localhost:6379 \
    -grpc-port $port \
    -insecure -debug \
    -endpoint-url http://localhost:4566 >/tmp/fsm-server-$port.log 2>&1 &
done

echo "Waiting for servers to come up..."
sleep 5

for i in $(seq 1 3)
do
  for port in $(seq 7494 7498)
  do
    fsmctl -addr localhost:$port -insecure Send data/order.yaml
  done
done
