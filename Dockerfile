# syntax=docker/dockerfile:1

# --- Base stage: download dependencies ---
FROM golang:1.26.1 AS base
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# --- Dev stage: linting and testing tools ---
FROM base AS dev
RUN go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.3

# --- Builder stage: compile binary ---
FROM base AS builder
RUN CGO_ENABLED=0 go build -buildvcs=false -o /app/alertmanager-webhook-relay .

# --- Production stage: minimal distroless image ---
FROM gcr.io/distroless/static-debian13:nonroot AS prod
COPY --from=builder /app/alertmanager-webhook-relay /alertmanager-webhook-relay
EXPOSE 8080
ENTRYPOINT ["/alertmanager-webhook-relay"]
