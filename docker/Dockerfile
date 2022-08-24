# Dockerfile for a simple Go application
#
# Created by M. Massenzio, 2022-04-28

# NOTE: Use the Ubuntu image, others will encounter errors,
#       including the "`GLIBC_2.32' not found" error.
FROM  ubuntu:22.04

RUN groupadd -r sm-bot && useradd -r -g sm-bot sm-bot
RUN apt-get update && apt-get install ca-certificates -y

WORKDIR /app

ADD docker/aws-credentials /home/sm-bot/.aws/credentials
COPY build/bin/sm-server docker/entrypoint.sh ./
RUN chmod a+x entrypoint.sh

USER sm-bot

# Fake AWS configuration to connect to LocalStack docker.
ENV AWS_REGION=us-west-2
ENV AWS_PROFILE=sm-bot

# Sensible defaults for the server
# See entrypoint.sh
ENV SERVER_PORT=7399
ENV EVENTS_Q=events
ENV ERRORS_Q=notifications
ENV REDIS=redis
ENV REDIS_PORT=6379
ENV DEBUG=""


EXPOSE ${SERVER_PORT}
ENTRYPOINT ["./entrypoint.sh"]
