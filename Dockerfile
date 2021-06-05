FROM golang:1.16.4-alpine3.12 AS builder
RUN mkdir /go/cron-restarter
WORKDIR /go/cron-restarter
COPY . .
RUN go build -o ./dist/restarter ./cmd/restarter

FROM alpine:3.12
RUN apk update \
    && apk add --no-cache \
    ca-certificates \
    && update-ca-certificates 2>/dev/null || true \
    && mkdir /app
WORKDIR /app
COPY --from=builder /go/cron-restarter/dist/restarter /app/restarter
