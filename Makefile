APP := ai-discord-bridge
CONFIG ?= config/config.toml
GO ?= go
FMT_FILES := $(shell rg --files -g '*.go')
LOG_DIR ?= /tmp/ai-discord-bridge
RUN_LOG ?= $(LOG_DIR)/run.log

.PHONY: build run test fmt lint clean

build:
	$(GO) build -o $(APP) .

run:
	@mkdir -p $(LOG_DIR)
	@nohup $(GO) run . -config $(CONFIG) > $(RUN_LOG) 2>&1 & echo $$! > $(LOG_DIR)/run.pid
	@echo "Started in background: PID $$(cat $(LOG_DIR)/run.pid)"
	@echo "Log: $(RUN_LOG)"

test:
	$(GO) test ./...

fmt:
	$(GO) fmt ./...

lint:
	@test -z "$$(gofmt -l $(FMT_FILES))" || (echo "gofmt check failed"; gofmt -l $(FMT_FILES); exit 1)
	$(GO) test ./...

clean:
	rm -f $(APP)
