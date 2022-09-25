FROM golang:1.17-alpine3.16 AS builder
WORKDIR /go/cron-restarter
COPY . .
RUN go build -o ./dist/restarter ./cmd/restarter

FROM alpine:3.16
RUN apk update \
    && apk add --no-cache \
    ca-certificates \
    && update-ca-certificates
COPY --from=builder /go/cron-restarter/dist/restarter /usr/local/bin/restarter
