.PHONY: c4 fmt vet lint security vulncheck dupl complexity coverage coverage-tools race integration mutation-pr mutation-full quality
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
	$(GO) test -tags=integration -count=1 -timeout=40m -covermode=atomic -coverpkg=./cmd/...,./internal/... -coverprofile=coverage.out ./cmd/... ./internal/...
	$(MAKE) coverage-tools
	@$(GO) tool cover -func=coverage.out
	@$(GO) run ./tools/coverage-gate -profile coverage.out -min 80 -critical-min 90
coverage-tools:
	$(GO) build ./tools/coverage-gate ./qa/driveauthprobe ./qa/dpm08probe
	$(GO) test ./tools/coverage-gate ./qa/driveauthprobe ./qa/dpm08probe
	python3 -m unittest discover -s qa -p 'test_coverage_report.py'
	$(GO) run ./qa/driveauthprobe -socket /nonexistent/bfe-auth-qr08.sock -expect unavailable

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

# Start or refresh the C4 viewer; stop only Structurizr occupying our port.
C4_PORT ?= 8080
.PHONY: c4 c4-stop
c4:
	@set -eu; \
	for id in $$(docker ps --filter "publish=$(C4_PORT)" --format '{{.ID}}'); do \
		image=$$(docker inspect --format '{{.Config.Image}}' "$$id"); \
		case "$$image" in \
			structurizr/structurizr|structurizr/structurizr:*) docker stop "$$id" >/dev/null ;; \
			*) echo "Port $(C4_PORT) is occupied by another application ($$image)" >&2; exit 1 ;; \
		esac; \
	done; \
	docker rm -f bfe-chat-c4 >/dev/null 2>&1 || true; \
	docker volume create bfe-structurizr-data >/dev/null; \
	docker run --rm -d --name bfe-chat-c4 \
		-p 127.0.0.1:$(C4_PORT):8080 \
		-v bfe-structurizr-data:/usr/local/structurizr \
		-v "$(CURDIR)/docs/architecture/c4/workspace.dsl:/usr/local/structurizr/workspace.dsl:ro" \
		structurizr/structurizr local >/dev/null; \
	echo "Structurizr: http://localhost:$(C4_PORT)"

c4-stop:
	@docker stop bfe-chat-c4
