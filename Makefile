APP := ai-discord-bridge
CONFIG ?= config/config.toml
GO ?= go
FMT_FILES := $(shell rg --files -g '*.go')
LOG_DIR ?= log
RUN_LOG ?= $(LOG_DIR)/run.log
RUN_PID ?= $(LOG_DIR)/run.pid
RUN_MATCH := ai-discord-bridge -config $(CONFIG)|go run \. -config $(CONFIG)

.PHONY: build run stop test fmt lint clean

build:
	$(GO) build -o $(APP) .

run:
	@mkdir -p $(LOG_DIR)
	@if [ -f $(RUN_PID) ]; then \
		pid=$$(cat $(RUN_PID)); \
		if kill -0 $$pid 2>/dev/null; then \
			echo "Stopping existing process: PID $$pid"; \
			kill $$pid; \
			wait $$pid 2>/dev/null || true; \
		fi; \
		rm -f $(RUN_PID); \
	fi
	@pids=$$(pgrep -f '$(RUN_MATCH)' || true); \
	if [ -n "$$pids" ]; then \
		echo "Stopping existing process(es): $$pids"; \
		kill $$pids; \
		sleep 1; \
	fi
	@$(GO) build -o $(APP) .
	@nohup ./$(APP) -config $(CONFIG) > $(RUN_LOG) 2>&1 & echo $$! > $(RUN_PID)
	@echo "Started in background: PID $$(cat $(RUN_PID))"
	@echo "Log: $(RUN_LOG)"

stop:
	@if [ -f $(RUN_PID) ]; then \
		pid=$$(cat $(RUN_PID)); \
		if kill -0 $$pid 2>/dev/null; then \
			echo "Stopping process: PID $$pid"; \
			kill $$pid; \
			wait $$pid 2>/dev/null || true; \
		else \
			echo "Stale PID file found: $$pid"; \
		fi; \
		rm -f $(RUN_PID); \
	else \
		echo "No PID file found"; \
	fi
	@pids=$$(pgrep -f '$(RUN_MATCH)' || true); \
	if [ -n "$$pids" ]; then \
		echo "Stopping matching process(es): $$pids"; \
		kill $$pids; \
	else \
		echo "No matching process found"; \
	fi

test:
	$(GO) test ./...

fmt:
	$(GO) fmt ./...

lint:
	@test -z "$$(gofmt -l $(FMT_FILES))" || (echo "gofmt check failed"; gofmt -l $(FMT_FILES); exit 1)
	$(GO) test ./...

clean:
	rm -f $(APP)
