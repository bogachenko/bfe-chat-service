.PHONY: update-runtime update-test-runtime update-test-nginx c4 fmt vet lint security vulncheck dupl complexity coverage coverage-tools race integration mutation-pr mutation-full quality
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

# C4 operations may touch only this checkout's explicitly labelled container.
C4_PORT ?= 8080
C4_NAME := bfe-chat-c4
C4_LABEL := com.bfe.chat.c4.workspace
.PHONY: c4 c4-stop
c4:
	@set -eu; \
	owner="$(CURDIR)"; \
	owned() { \
		actual=$$(docker inspect --format '{{ index .Config.Labels "$(C4_LABEL)" }}' "$$1") || return 1; \
		[ "$$actual" = "$$owner" ]; \
	}; \
	name_id=$$(docker ps -aq --filter 'name=^/$(C4_NAME)$$') || exit 1; \
	if [ -n "$$name_id" ] && ! owned "$$name_id"; then \
		echo "Container name $(C4_NAME) is owned by another project" >&2; \
		exit 1; \
	fi; \
	port_ids=$$(docker ps -q --filter "publish=$(C4_PORT)") || exit 1; \
	for id in $$port_ids; do \
		if [ "$$id" != "$$name_id" ]; then \
			echo "Port $(C4_PORT) belongs to another container ($$id)" >&2; \
			exit 1; \
		fi; \
		if ! owned "$$id"; then \
			echo "Port $(C4_PORT) belongs to an unowned container ($$id)" >&2; \
			exit 1; \
		fi; \
	done; \
	if [ -n "$$name_id" ]; then \
		docker stop "$$name_id" >/dev/null; \
		docker rm "$$name_id" >/dev/null; \
	fi; \
	docker volume create bfe-structurizr-data >/dev/null; \
	docker run -d --name "$(C4_NAME)" \
		--label "$(C4_LABEL)=$$owner" \
		-p 127.0.0.1:$(C4_PORT):8080 \
		-v bfe-structurizr-data:/usr/local/structurizr \
		-v "$(CURDIR)/docs/architecture/c4/workspace.dsl:/usr/local/structurizr/workspace.dsl:ro" \
		structurizr/structurizr local >/dev/null; \
	echo "Structurizr: http://localhost:$(C4_PORT)"

c4-stop:
	@set -eu; \
	id=$$(docker ps -aq --filter 'name=^/$(C4_NAME)$$') || exit 1; \
	if [ -z "$$id" ]; then \
		echo "No $(C4_NAME) container"; \
	else \
		owner=$$(docker inspect --format '{{ index .Config.Labels "$(C4_LABEL)" }}' "$$id") || exit 1; \
		if [ "$$owner" != "$(CURDIR)" ]; then \
			echo "Refusing to stop unowned container $(C4_NAME)" >&2; \
			exit 1; \
		fi; \
		docker stop "$$id" >/dev/null; \
		docker rm "$$id" >/dev/null; \
	fi

# Production Chat deployment is not configured.
update-runtime:
	@echo 'ERROR: production Chat deployment is not configured or authorized' >&2; exit 1

update-test-runtime:
	@test -n "$(BFE_TEST_VPS_SSH_TARGET)" || { echo 'BFE_TEST_VPS_SSH_TARGET is required' >&2; exit 1; }
	@test -n "$(BFE_TEST_PUBLIC_HOST)" || { echo 'BFE_TEST_PUBLIC_HOST is required' >&2; exit 1; }
	@bash ./deploy/scripts/update-test-runtime.sh

# Apply only repository-managed dedicated test Chat nginx vhost.
update-test-nginx:
	@bash ./deploy/scripts/update-test-nginx.sh
