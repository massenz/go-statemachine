# Copyright AlertAvert.com (c) 2023
# Created by M. Massenzio, 2022-04-28

FROM golang:1.19 as builder

WORKDIR /server
COPY . .
RUN rm -rf certs data
RUN CGO_ENABLED=0 make build
RUN CGO_ENABLED=0 go build -o out/bin/hc docker/grpc_health.go

# NOTE: Use the Ubuntu image; other images will encounter errors,
#       including the "`GLIBC_2.32' not found" error.
#       Alpine (which uses /bin/sh) emits an error which makes no sense (./fsm-server not found)
#       only because, in fact, it cannot find the library.
FROM  ubuntu:22.04
LABEL org.opencontainers.image.authors="Marco Massenzio <marco@alertavert.com>"
LABEL org.opencontainers.image.source=https://github.com/massenz/go-statemachine
LABEL org.opencontainers.image.description="Statemachine gRPC Server"
LABEL org.opencontainers.image.licenses=Apache-2.0

RUN apt-get update && apt-get install ca-certificates -y
RUN groupadd -r sm-bot && useradd -r -g sm-bot sm-bot

# Sensible defaults for the server, for reference
# we list all the environment variables used by the
# entrypoint script.
ENV GRPC_PORT=7398 SERVER_PORT=7399 DEBUG=""  \
    EVENTS_Q="events" NOTIFICATIONS_Q="notifications" \
    ACKS_Q="" ERRORS_ONLY=t \
    CLUSTER="" REDIS="redis:6379" \
    TIMEOUT=500ms INSECURE=""

WORKDIR /app
RUN chown sm-bot:sm-bot /app

USER sm-bot
COPY --from=builder /server/out/bin/fsm-server* ./fsm-server
COPY --from=builder /server/out/bin/hc ./
ADD docker/entrypoint.sh ./

EXPOSE ${SERVER_PORT}
ENTRYPOINT ["./entrypoint.sh"]
HEALTHCHECK --start-period=5s --interval=30s --timeout=200ms --retries=5 \
  CMD ./hc -host localhost:$GRPC_PORT -timeout $TIMEOUT $INSECURE
