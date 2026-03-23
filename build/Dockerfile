FROM golang:1.25.1-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /subscriptions ./cmd/subscriptions

FROM alpine:3.22
WORKDIR /app
RUN adduser -D appuser
COPY --from=builder /subscriptions /app/subscriptions
COPY configs/config.yml /app/configs/config.yml
RUN mkdir -p /app/var/log/subscriptions && chown -R appuser:appuser /app
USER appuser
EXPOSE 8080
ENTRYPOINT ["/app/subscriptions", "-config", "/app/configs/config.yml"]
