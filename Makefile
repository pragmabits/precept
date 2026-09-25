# Installs the analyzers of this module, standalone or built into golangci-lint,
# and runs the gate. `make` alone lists the targets.

GOLANGCI_LINT ?= golangci-lint

# Where `go install` puts binaries: GOBIN, or the bin of the first GOPATH entry.
bin := $(or $(shell go env GOBIN),$(firstword $(subst :, ,$(shell go env GOPATH)))/bin)

.DEFAULT_GOAL := help

.PHONY: help install golangci test lint format

help: ## List the targets.
	@echo "usage: make <target>"
	@echo
	@awk 'BEGIN { FS = ":.*## " } /^[a-z]+:.*## / { printf "  %-10s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

install: ## Install the command of every analyzer into GOBIN.
	go install ./cmd/...

golangci: ## Build golangci-lint with the plugins, as golangci-lint-precept in GOBIN.
	$(GOLANGCI_LINT) custom --destination $(bin)

test: ## Run the tests, with the race detector, as the CI does.
	go test -race ./...

lint: ## Run the linters, as the CI does.
	go mod tidy -diff
	$(GOLANGCI_LINT) config verify
	$(GOLANGCI_LINT) run ./...
	$(GOLANGCI_LINT) run --tests=false --enable-only unused ./...

format: ## Rewrite the files the formatters of .golangci.yml would change.
	$(GOLANGCI_LINT) fmt
