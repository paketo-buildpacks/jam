FROM alpine:3.15.4
ARG packages
RUN apk add --no-cache ${packages}
