# Dockerfile for a simple Go application
#
# Created by M. Massenzio, 2022-04-28

# NOTE: Use the Ubuntu image, others will encounter errors,
#       including the "`GLIBC_2.32' not found" error.
FROM  ubuntu:22.04

RUN apt-get update && apt-get install ca-certificates -y
RUN groupadd -r sm-bot && useradd -r -g sm-bot sm-bot

# Sensible defaults for the server, for reference
# we list all the environment variables used by the
# entrypoint script.
#
ENV GRPC_PORT=7398 SERVER_PORT=7399 DEBUG=""  \
    EVENTS_Q="events" NOTIFICATIONS_Q="notifications" \
    ACKS_Q="" ERRORS_ONLY=1 \
    CLUSTER="" REDIS=redis REDIS_PORT=6379

WORKDIR /app
RUN chown sm-bot:sm-bot /app

USER sm-bot
ADD build/bin/sm-server docker/entrypoint.sh ./

EXPOSE ${SERVER_PORT}
ENTRYPOINT ["./entrypoint.sh"]
