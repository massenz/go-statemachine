# Copyright AlertAvert.com (c) 2023
# Created by M. Massenzio, 2022-04-28
#
# NOTE: Use the Ubuntu image; other images will encounter errors,
#       including the "`GLIBC_2.32' not found" error.
#       Alpine (which uses /bin/sh) emits an error which makes no sense (./sm-server not found)
#       only because, in fact, it cannot find the library.
FROM  ubuntu:22.04
LABEL org.opencontainers.image.authors="Marco Massenzio <marco@alertavert.com>"
LABEL org.opencontainers.image.source=https://github.com/massenz/go-statemachine
LABEL org.opencontainers.image.description="Statemachine gRPC Server"
LABEL org.opencontainers.image.licenses=Apache-2.0

ARG appname

#RUN apt-get update && apt-get install ca-certificates -y
RUN groupadd -r sm-bot && useradd -r -g sm-bot sm-bot

# Sensible defaults for the server, for reference
# we list all the environment variables used by the
# entrypoint script.
#
ENV GRPC_PORT=7398 SERVER_PORT=7399 DEBUG=""  \
    EVENTS_Q="events" NOTIFICATIONS_Q="notifications" \
    ACKS_Q="" ERRORS_ONLY=t \
    CLUSTER="" REDIS="redis:6379"

WORKDIR /app
RUN chown sm-bot:sm-bot /app

USER sm-bot
ADD $appname ./sm-server
ADD docker/entrypoint.sh ./

EXPOSE ${SERVER_PORT}
ENTRYPOINT ["./entrypoint.sh"]
