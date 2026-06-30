# syntax=docker/dockerfile:1

# --- Base stage: download dependencies ---
FROM golang:1.26.4 AS base
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# --- Dev stage: linting and testing tools ---
FROM base AS dev
RUN go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2

# --- Builder stage: compile binary ---
FROM base AS builder
RUN CGO_ENABLED=0 go build -buildvcs=false -o /app/alertmanager-webhook-relay .

# --- Production stage: minimal distroless image ---
FROM gcr.io/distroless/static-debian13:nonroot AS prod

LABEL org.opencontainers.image.title="alertmanager-webhook-relay" \
      org.opencontainers.image.description="Receives alerts from Prometheus Alertmanager and forwards them to notification channels" \
      org.opencontainers.image.source="https://github.com/Etherlord/alertmanager-webhook-relay" \
      org.opencontainers.image.licenses="MIT"

COPY --from=builder /app/alertmanager-webhook-relay /alertmanager-webhook-relay
COPY --from=builder /app/templates /templates

EXPOSE 8080
ENTRYPOINT ["/alertmanager-webhook-relay"]
