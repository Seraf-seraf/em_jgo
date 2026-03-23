APP_NAME := subscriptions
CMD_PATH := ./cmd/subscriptions
CONFIG ?= configs/config.yml
DOCKER_COMPOSE ?= docker compose
DOCKERFILE := build/Dockerfile
UNIT_PACKAGES := $(shell go list ./... | grep -v '^github.com/example/em_jgo/internal/repository/postgres$$')

.PHONY: build run test test-unit test-integration fmt up down compose-up compose-down docker-build generate

build:
	mkdir -p bin
	go build -o bin/$(APP_NAME) $(CMD_PATH)

run:
	go run $(CMD_PATH) -config $(CONFIG)

fmt:
	go fmt ./...

test: test-unit test-integration

test-unit:
	go test $(UNIT_PACKAGES)

test-integration:
	go test ./internal/repository/postgres -count=1

generate:
	go tool oapi-codegen -config api/oapi-codegen.yaml api/openapi.yaml

docker-build:
	docker build -f $(DOCKERFILE) -t $(APP_NAME):local .

up:
	$(DOCKER_COMPOSE) up --build

down:
	$(DOCKER_COMPOSE) down -v

compose-up:
	$(MAKE) up

compose-down:
	$(MAKE) down
