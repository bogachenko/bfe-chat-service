.PHONY: fmt vet lint security vulncheck dupl complexity coverage race integration mutation-pr mutation-full quality

GO ?= go
GOFILES := $(shell find cmd internal -type f -name '*.go' ! -name '*_test.go' 2>/dev/null)

fmt:
	@test -z "$$(gofmt -l $(GOFILES))" || { gofmt -l $(GOFILES); exit 1; }

vet:
	$(GO) vet ./...

lint:
	golangci-lint run ./...

security:
	gosec ./...

vulncheck:
	govulncheck ./...

dupl:
	@test -z "$$(dupl -t 100 $(GOFILES))" || { dupl -t 100 $(GOFILES); exit 1; }

complexity:
	@cyclop -over 10 ./...

coverage:
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out

race:
	$(GO) test -race ./...

integration:
	$(GO) test -tags=integration ./...

mutation-pr:
	gremlins unleash

mutation-full:
	gremlins unleash

quality: fmt vet lint security vulncheck dupl complexity coverage race
