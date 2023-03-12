# syntax=docker/dockerfile:experimental

FROM alpine:3.15.4

ARG packages
ARG platform

LABEL platform=${platform}

RUN apk add --no-cache ${packages}

RUN --mount=type=secret,id=some-secret,dst=/some-secret \
  cat /some-secret > /my-secret
