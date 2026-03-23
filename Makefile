APP_NAME := subscriptions
CMD_PATH := ./cmd/subscriptions
CONFIG ?= configs/config.yml
DOCKERFILE := build/Dockerfile

.PHONY: build run test fmt up down generate

build:
	mkdir -p bin
	go build -o bin/$(APP_NAME) $(CMD_PATH)

run:
	go run $(CMD_PATH) -config $(CONFIG)

fmt:
	go fmt ./...

test:
	go test ./...

generate:
	cp api/openapi.yaml internal/pkg/transport/http/openapi.yaml
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.6.0 -config api/oapi-codegen.yaml api/openapi.yaml

up:
	docker compose up --build -d

down:
	docker compose down -v
