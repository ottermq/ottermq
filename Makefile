BINARY_NAME=ottermq
ADMIN_BINARY_NAME=ottermqadmin
BUILD_DIR=bin
MAIN_PATH=cmd/ottermq/main.go
ADMIN_MAIN_PATH=cmd/ottermqadmin/main.go

PREFIX ?= /usr/local
BINDIR ?= $(PREFIX)/bin

build:
	@mkdir -p $(BUILD_DIR)
	@VERSION=$$(git describe --tags --always 2>/dev/null || echo dev); \
	go build -ldflags "-X main.VERSION=$$VERSION" -o ./$(BUILD_DIR)/$(BINARY_NAME) ./$(MAIN_PATH)

build-admin:
	@mkdir -p $(BUILD_DIR)
	@go build -o ./$(BUILD_DIR)/$(ADMIN_BINARY_NAME) ./$(ADMIN_MAIN_PATH)

docs:
	@GOPATH=$$(go env GOPATH); \
	$$GOPATH/bin/swag init -g ../../../$(MAIN_PATH) --pd -d web/handlers/api,web/handlers/api_admin -exclude web/handlers/webui/ -o ./web/docs --ot go

install: build
	@mkdir -p $(BINDIR)
	@install -m 0755 ./$(BUILD_DIR)/$(BINARY_NAME) $(BINDIR)/$(BINARY_NAME)

install-admin: build-admin
	@mkdir -p $(BINDIR)
	@install -m 0755 ./$(BUILD_DIR)/$(ADMIN_BINARY_NAME) $(BINDIR)/$(ADMIN_BINARY_NAME)

run: build
	@./$(BUILD_DIR)/$(BINARY_NAME)

test:
	@go test ./... -v

test-cli:
	@go test ./cmd/ottermqadmin ./internal/cli ./pkg/adminapi/client -v

lint:
	@golangci-lint run

ui-deps:
	@cd ottermq_ui && npm install

ui-build: ui-deps
	@cd ottermq_ui && npx quasar build
	@rm -rf ui
	@cp -r ottermq_ui/dist/spa ui

build-all: ui-build build build-admin

run-dev: build
	@echo "UI running separately at http://localhost:9000"
	@echo "Starting broker..."
	@./$(BUILD_DIR)/$(BINARY_NAME)

clean:
	@rm -f $(BUILD_DIR)/$(BINARY_NAME)
	@rm -f $(BUILD_DIR)/$(ADMIN_BINARY_NAME)
	@rm -rf ./ui
	@rm -rf ./ottermq_ui/dist

.PHONY: build build-admin install install-admin clean run docs test test-cli lint ui-deps ui-build build-all run-dev
