.PHONY: all fmt lint build

BINARY := electrorangerd
BIN_DIR := bin

all: fmt lint build

fmt:
	go fmt ./...

lint:
	golangci-lint run ./...

build:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(BINARY) ./cmd/electrorangerd
