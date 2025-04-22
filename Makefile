GOLANGCI_LINT_VERSION ?= v1.64.8

.PHONY: deps
deps: tidy verify

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: verify
verify:
	go mod verify


.PHONY: lint
lint:
	go run github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run --allow-parallel-runners -c .golangci.yml
