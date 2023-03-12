FROM alpine:3.15.4
ARG packages
ARG platform
LABEL platform=${platform}
RUN apk add --no-cache ${packages}
