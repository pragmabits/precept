# Installs the analyzers of this module, standalone or built into golangci-lint,
# and runs the gate. `make` alone lists the targets.

GOLANGCI_LINT ?= golangci-lint

# Where `go install` puts binaries: GOBIN, or the bin of the first GOPATH entry.
bin := $(or $(shell go env GOBIN),$(firstword $(subst :, ,$(shell go env GOPATH)))/bin)

# golangci-lint custom clones golangci-lint by the tag in plugin/.custom-gcl.yml,
# and a tag can be moved to other code: the build runs only while the tag still
# names this commit. A new version there needs its commit here. The seconds
# between the check and the clone are not covered.
golangci_version := $(shell sed -n 's/^version: //p' plugin/.custom-gcl.yml)
golangci_commit := 114493f9b3e7257d29e4130f2b4a4aadefbb6845

.DEFAULT_GOAL := help

.PHONY: help install golangci test e2e lint format

help: ## List the targets.
	@echo "usage: make <target>"
	@echo
	@awk 'BEGIN { FS = ":.*## " } /^[a-z0-9]+:.*## / { printf "  %-10s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

install: ## Install the command of every analyzer where go install puts it.
	go install ./cmd/...

# golangci-lint custom reads .custom-gcl.yml only from the directory it runs in.
golangci: ## Build golangci-lint with the plugins, as golangci-lint-precept in GOBIN.
	@tagged=$$(git ls-remote https://github.com/golangci/golangci-lint.git \
		'refs/tags/$(golangci_version)^{}' | cut -f1); \
	if [ "$$tagged" != "$(golangci_commit)" ]; then \
		echo "golangci-lint $(golangci_version) names $${tagged:-no commit}, not $(golangci_commit)" >&2; \
		exit 1; \
	fi
	cd plugin && $(GOLANGCI_LINT) custom --destination $(bin)

# e2e/ builds cmd/paircheck, which it does not import, and the test cache
# cannot see that code change: the tests always run.
test: ## Run the tests, with the race detector, as the CI does.
	go test -race -count=1 ./...

# The golangci-lint half of the end-to-end tests needs golangci-lint built with
# the plugins, which clones golangci-lint: it sits behind the golangci tag. The
# test cache cannot see that binary change, so the tests always run.
e2e: golangci ## Run the end-to-end tests, golangci-lint with the plugins included.
	PRECEPT_GOLANGCI_LINT=$(bin)/golangci-lint-precept go test -count=1 -tags golangci ./e2e/

lint: ## Run the linters, as the CI does.
	go mod tidy -diff
	$(GOLANGCI_LINT) config verify
	$(GOLANGCI_LINT) run ./...
	$(GOLANGCI_LINT) run --tests=false --enable-only unused ./...

format: ## Rewrite the files the formatters of .golangci.yml would change.
	$(GOLANGCI_LINT) fmt
