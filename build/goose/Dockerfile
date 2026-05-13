# syntax=docker/dockerfile:1

# Minimal goose image built from official source.
# Replaces the non-existent ghcr.io/pressly/goose image.

ARG GOOSE_VERSION=v3.27.1

FROM golang:1.26.3-alpine AS builder
ARG GOOSE_VERSION
RUN go install github.com/pressly/goose/v3/cmd/goose@${GOOSE_VERSION}

FROM alpine:3
# hadolint ignore=DL3018
RUN apk add --no-cache ca-certificates
COPY --from=builder /go/bin/goose /bin/goose

ENTRYPOINT ["goose"]
