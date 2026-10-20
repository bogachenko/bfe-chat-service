# BFE Chat Service — Engineering Quality Requirements

**Status:** Existing project requirements, recovered from repository history and recorded here without weakening or inventing acceptance thresholds.  
**Applies to:** All production changes, including Issue #25 (`internal/issue25`).  
**Scope:** Engineering verification; not an authorization to deploy, migrate remote databases, or modify BFE Drive.

## Normative sources (already committed)

- [Makefile](../Makefile): executable targets `fmt`, `vet`, `lint`, `security`, `vulncheck`, `dupl`, `complexity`, `coverage`, `race`, `integration`, `mutation-pr`, `mutation-full`, `quality`.
- [GitHub Actions Quality Gates](../.github/workflows/quality.yml): `make quality` on PR and main push; `make mutation-pr` on PR.
- [Coverage gate](../tools/coverage-gate/main.go): production statement coverage calculation and named critical packages.
- Historical evidence: [de95d32](https://github.com/bogachenko/bfe-chat-service/commit/de95d32b26498343e615f65a877668607811a222) (deduplicated production coverage and thresholds), [1696497](https://github.com/bogachenko/bfe-chat-service/commit/1696497d710ea18e69e157738597e854c2421d1c) (staged QA is not release approval), [220469d](https://github.com/bogachenko/bfe-chat-service/commit/220469d166b1a8ae72ae7e5cc8b625603b053ecd) (global gate blocked notwithstanding module-level success).

## Required project quality gates

| ID | Check | Existing command / acceptance requirement |
| --- | --- | --- |
| QR-FMT | Go formatting | `make fmt`: no files reported by `gofmt -l` for production Go. |
| QR-VET | Static correctness | `make vet`: `go vet ./...` PASS. |
| QR-LINT | Lint | `make lint`: `golangci-lint run ./...` PASS. |
| QR-SEC | Security | `make security`: `gosec -severity high -confidence medium ./cmd/... ./internal/... ./tools/...` PASS. |
| QR-VULN | Dependencies | `make vulncheck`: `govulncheck ./...` PASS. |
| QR-DUPL | Duplication | `make dupl`: `dupl -t 100` reports **0 clone groups**. |
| QR-COMPLEX | Cyclomatic complexity | `make complexity`: `cyclop -maxComplexity 10 ./...` PASS. |
| QR-COVER | Production statement coverage | `make coverage`: **>=80% overall**, **>=90% separately for each named critical package**. The existing gate names `internal/authentication` and `internal/authorization`. |
| QR-RACE | Go race detector | `make race`: `go test -race ./...` PASS. |
| QR-INTEGRATION | Integration tests | `make integration`: `go test -tags=integration ./...` PASS; project coverage also uses the `integration` tag. |
| QR-MUTATION | Changed-code mutation testing | `make mutation-pr` with go-gremlins on PR (`make mutation-full` for full run); report killed/lived/not-covered mutations and exact scope. **No numeric minimum mutation score has been found in the verified historical gates; do not invent one or report a numeric mutation PASS threshold.** |
| QR-ALL | Aggregated blocking gate | `make quality` includes fmt, vet, lint, security, vulncheck, dupl, complexity, coverage and race; PR CI additionally executes changed-code mutation. |

**Testing evidence:** `go build ./...` and `go test ./...` passing alone do **not** establish `QR-ALL`, integration, mutation adequacy, or release acceptance. Preserve reproducible job URL, commit SHA, measured production coverage by package, mutation outcomes, race/security/lint logs, integration results, and failed or excluded checks. A module-level or staged exception does not waive project thresholds or authorize production release.

**Coverage methodology:** `go test -tags=integration -count=1 -covermode=atomic -coverpkg=./cmd/...,./internal/... -coverprofile=coverage.out ./cmd/... ./internal/...` then `go run ./tools/coverage-gate -profile coverage.out -min 80 -critical-min 90`. Count each unique coverage block once (merge duplicate block hit information); QA and tools are verified separately, not mixed into production coverage. Critical-package policy changes require explicit project approval.

## Issue #25 — mandatory gap to correct, not an exception

At the time of this documentation the existing `tools/coverage-gate/main.go` uses a hard-coded production package list that **does not include `internal/issue25`**. Yet `internal/issue25` is production code. Because the profile parser rejects unknown production packages, the current gate cannot validly certify Issue #25 production coverage as-is. This is a **blocking defect in gate scope**, not grounds for excluding code or reducing thresholds.

Before claiming Issue #25 Quality Gate PASS:
1. Amend the coverage gate and its tests so **all** production packages, including `internal/issue25`, are counted exactly once; retain the existing **80% overall / 90% named-critical** requirements without silently changing the critical list.
2. Ensure the Issue #25 branch runs the **full existing quality workflow**, not merely the narrower `issue25-validation.yml` workflow (which currently runs build, migrations and `go test ./...`). A separate documented workflow is not a substitute for a required gate.
3. Provide results of mandatory mutation testing, race, integration/real PostgreSQL checks and static/security checks on the exact reviewed SHA.
4. Map approved Issue #25 requirements/contracts to actual automated test assertions and results for `MS25-01` through `MS25-15`; `NOT COVERED` is not PASS.
5. Independent QA reviews evidence before acceptance. No deployment, remote DB changes, or BFE Drive changes are implied by this document.

**Source of authority:** This document records pre-existing repository rules and an observed Issue #25 enforcement gap. It introduces **no new numeric thresholds**. Any future change of policy requires a separately reviewed, explicit Project Owner decision.
