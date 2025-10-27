MKFILE_PATH := $(abspath $(lastword $(MAKEFILE_LIST)))
PROJECT_PATH := $(patsubst %/,%,$(dir $(MKFILE_PATH)))
LOCAL_BIN_PATH := ${PROJECT_PATH}/bin

LINT_GOGC := 10
LINT_TIMEOUT := 10m

## Tools
GOLANGCI_VERSION ?= v2.5.0
GOLANGCI ?= go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_VERSION)
GOVULNCHECK ?= go run golang.org/x/vuln/cmd/govulncheck@latest

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif


ifndef ignore-not-found
  ignore-not-found = false
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: help

.PHONY: help
help: ## Display this help screen
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: clean
clean: ## Clean up build artifacts and test cache
	go clean -x
	go clean -x -testcache

.PHONY: fmt
fmt: ## Format Go source code
	@$(GOLANGCI) fmt --config .golangci.yml
	go fmt ./...

.PHONY: test
test: ## Run all tests
	go test -v ./...

.PHONY: test/race
test/race: ## Run tests with the race detector
	go test -race ./...

.PHONY: test/cover
test/cover: ## Run tests and generate a coverage report
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

.PHONY: deps
deps: ## Tidy Go module dependencies
	go mod tidy

.PHONY: lint
lint: ## Run linter
	@$(GOLANGCI) run --config .golangci.yml --timeout $(LINT_TIMEOUT)

.PHONY: lint/fix
lint/fix: ## Run linter and apply fixes
	@$(GOLANGCI) run --config .golangci.yml --timeout $(LINT_TIMEOUT) --fix

.PHONY: vuln
vuln: ## Run vulnerability check
	@$(GOVULNCHECK) ./...

LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	@mkdir -p $(LOCALBIN)

