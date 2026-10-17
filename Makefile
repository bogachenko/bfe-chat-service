.PHONY: fmt vet lint security vulncheck dupl complexity coverage race integration mutation-pr mutation-full quality
GO ?= go
GOFILES := $(shell find cmd internal -type f -name '*.go' ! -name '*_test.go' 2>/dev/null)
BASE_REF ?= origin/main

fmt:
	@test -z "$$(gofmt -l $(GOFILES))" || { gofmt -l $(GOFILES); exit 1; }
vet:
	$(GO) vet ./...
lint:
	golangci-lint run ./...
security:
	gosec -severity high -confidence medium ./cmd/... ./internal/... ./tools/...
vulncheck:
	govulncheck ./...
dupl:
	@bash -o pipefail -c 'dupl -t 100 $(GOFILES) | tee /dev/stderr | grep -Fxq "Found total 0 clone groups."'
complexity:
	@cyclop -maxComplexity 10 ./...
coverage:
	$(GO) test -tags=integration -timeout=40m -coverprofile=coverage.out ./...
	@$(GO) tool cover -func=coverage.out
	@$(GO) run ./tools/coverage-gate -profile coverage.out -min 80 -critical-min 90
race:
	$(GO) test -race ./...
integration:
	$(GO) test -tags=integration ./...
mutation-pr:
	@test -n "$(BASE_REF)" || { echo BASE_REF required; exit 1; }
	gremlins unleash --diff "$(BASE_REF)" --exclude-files '.*_test\.go$$' --exclude-files '.*\.pb\.go$$'
mutation-full:
	gremlins unleash --exclude-files '.*_test\.go$$' --exclude-files '.*\.pb\.go$$'
quality: fmt vet lint security vulncheck dupl complexity coverage race
