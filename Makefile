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
	gosec -severity high -confidence medium ./...
vulncheck:
	govulncheck ./...
dupl:
	@output="$$(dupl -t 100 $(GOFILES))"; test -z "$$output" || { printf '%s\n' "$$output"; exit 1; }
complexity:
	@cyclop -maxComplexity 10 ./...
coverage:
	$(GO) test -coverprofile=coverage.out ./...
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
