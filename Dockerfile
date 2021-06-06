FROM golang:1.16-alpine3.12 AS builder
WORKDIR /go/cron-restarter
COPY . .
RUN go build -o ./dist/restarter ./cmd/restarter

FROM alpine:3.12
RUN apk update \
    && apk add --no-cache \
    ca-certificates \
    && update-ca-certificates
COPY --from=builder /go/cron-restarter/dist/restarter /usr/local/bin/restarter
