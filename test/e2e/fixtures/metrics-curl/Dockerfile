FROM alpine:3.23
RUN apk add --no-cache curl \
    && adduser -D -u 1000 -g 1000 metrics
USER 1000
