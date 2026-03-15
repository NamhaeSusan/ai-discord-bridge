APP := ai-discord-bridge
CONFIG ?= config/config.toml
GO ?= go
FMT_FILES := $(shell rg --files -g '*.go')

.PHONY: build run test fmt lint clean

build:
	$(GO) build -o $(APP) .

run:
	$(GO) run . -config $(CONFIG)

test:
	$(GO) test ./...

fmt:
	$(GO) fmt ./...

lint:
	@test -z "$$(gofmt -l $(FMT_FILES))" || (echo "gofmt check failed"; gofmt -l $(FMT_FILES); exit 1)
	$(GO) test ./...

clean:
	rm -f $(APP)
