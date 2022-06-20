# Dockerfile for a simple Go application
#
# Created by M. Massenzio, 2022-04-28

FROM  debian:stable-slim
WORKDIR /app

COPY build/bin/sm-server docker/entrypoint.sh ./

RUN groupadd -r sm-bot && useradd -r -g sm-bot sm-bot
ADD docker/aws-credentials /home/sm-bot/.aws/credentials
RUN chmod +x entrypoint.sh
RUN chown sm-bot:sm-bot entrypoint.sh

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
