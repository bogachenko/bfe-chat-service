# BFE Chat Service — Engineering Quality Requirements

**Status:** Existing project requirements, recovered from repository history and recorded here without weakening or inventing acceptance thresholds.  
**Applies to:** All production changes, including Issue #25 (`internal/issue25`).  
**Scope:** Engineering verification; not an authorization to deploy, migrate remote databases, or modify BFE Drive.

## Primary approved normative source

**[QualityRequirements.xml](../QualityRequirements.xml)** — `QR-BFE-CHAT-001`, `status="HUMAN_APPROVED"`, dated 2026-10-16, originally committed as [50876d9](https://github.com/bogachenko/bfe-chat-service/commit/50876d921249dd53080908806cee575e7459f542). **This XML is the source of truth.** `Makefile`, CI and this Markdown are implementations/explanations of its requirements, not alternative policy. Where commands do not fully enforce XML requirements, record a gap instead of silently narrowing requirements.

## Normative sources (already committed)

- [Makefile](../Makefile): executable targets `fmt`, `vet`, `lint`, `security`, `vulncheck`, `dupl`, `complexity`, `coverage`, `race`, `integration`, `mutation-pr`, `mutation-full`, `quality`.
- [GitHub Actions Quality Gates](../.github/workflows/quality.yml): `make quality` on PR and main push; `make mutation-pr` on PR.
- [Coverage gate](../tools/coverage-gate/main.go): production statement coverage calculation and named critical packages.
- Historical evidence: [de95d32](https://github.com/bogachenko/bfe-chat-service/commit/de95d32b26498343e615f65a877668607811a222) (deduplicated production coverage and thresholds), [1696497](https://github.com/bogachenko/bfe-chat-service/commit/1696497d710ea18e69e157738597e854c2421d1c) (staged QA is not release approval), [220469d](https://github.com/bogachenko/bfe-chat-service/commit/220469d166b1a8ae72ae7e5cc8b625603b053ecd) (global gate blocked notwithstanding module-level success).

## Approved requirements in QualityRequirements.xml — complete inventory

**Code quality:** `QR-01` gofmt zero deviations; `QR-02` go vet zero findings; `QR-03` pinned golangci-lint config, zero findings; `QR-04` gosec with **every** finding reviewed and documented and no critical/high risks admitted to release (the current `make security` command checks only high severity and above and does not by itself evidence review of all findings); `QR-05` zero known applicable vulnerabilities and tool failure blocks; `QR-06` detect handmade-code duplication from 100 tokens and analyze/remediate findings (current Makefile demands 0 clone groups); `QR-07` cyclomatic complexity max 10; `QR-08` >=80% overall and >=90% for **Auth, DPoP and AccessControl code**, with scope/exclusions explicit (current coverage gate names only authentication and authorization; verify complete mapping of all DPoP/AccessControl code); `QR-09` zero races. **All gates apply from the first production implementation commit, locally and in CI, using pinned versions.**

**Mandatory QA categories:** `QA-01` unit/business validation/permissions; `QA-02` real PostgreSQL, BFE Auth UDS/DPoP and Centrifuge integration; `QA-03` Flutter Web/Android ↔ Chat Service ↔ PostgreSQL E2E; `QA-04` security: DPoP signature/key binding, `ath`, freshness, replay/reconnect, revoked tokens, ADMIN separation and history isolation; `QA-05` concurrent reassignment, send/assignment race, post-change online delivery and concurrent idempotency; `QA-06` BFE Auth/PostgreSQL outages, rollback, lost response after commit and Centrifuge outage; `QA-07` latency/concurrent connections/load, with numerical operating budgets set before release from measurements/VPS; `QA-08` regression of defects and critical invariants; `QA-09` **rapid** property-based, state-machine and model-based checks; `QA-10` behavioral mutation testing.

**Approved test infrastructure:** `testcontainers-go`, isolated auto-cleaned Docker containers with real PostgreSQL **18.6**, actual BFE Drive Internal Auth API via UDS/DPoP, real Go Chat Service/Centrifuge. Pin a concrete SHA from BFE Drive `experimental/internal-auth-api` for reproducible read-only integration; mocks are permitted for unit tests, not substitutes for critical integrations. Read-only source/test usage does not authorize modifying BFE Drive or production infrastructure.

**Mutation policy:** go-gremlins **v0.6.0**, handwritten production Go only (exclude tests/generated code), every PR for affected code, tool errors block CI, publish `killed`, `survived`, `not covered`, `mutation coverage`, `test efficacy`; analyze survivors. Repository-wide mutation periodically/on request. **No numerical score threshold approved:** establish baseline first and seek separate approval for any threshold; do not fail solely on a numeric score.

**Automation:** `CI-01` consistent Makefile targets; `CI-02` required GitHub Actions checks for every PR **plus branch protection preventing merge on failure** (workflow existence alone does not prove branch protection); `CI-03` pinned tool/runtime/container versions; `CI-04` isolated reproducible tests with **no production data or production services**; `CI-05` no silent exceptions/waivers.

**Release criteria:** `PR-01` all quality gates PASS; `PR-02` all applicable unit/integration/E2E/security/concurrency/failure/performance/regression PASS; `PR-03` no known critical bugs/high-risk security issues; `PR-04` demonstrated DPoP/replay/reconnect, idempotency, transaction consistency, no unauthorized delivery; `PR-05` reproducible builds/tests and proven existing VPS/runtime readiness before deployment. These remain release requirements, not permission to deploy.

## Existing executable project quality gates

| ID | Check | Existing command / acceptance requirement |
| --- | --- | --- |
| QR-FMT | Go formatting | `make fmt`: no files reported by `gofmt -l` for production Go. |
| QR-VET | Static correctness | `make vet`: `go vet ./...` PASS. |
| QR-LINT | Lint | `make lint`: `golangci-lint run ./...` PASS. |
| QR-SEC | Security | `make security`: `gosec -severity high -confidence medium ./cmd/... ./internal/... ./tools/...` PASS. |
| QR-VULN | Dependencies | `make vulncheck`: `govulncheck ./...` PASS. |
| QR-DUPL | Duplication | `make dupl`: `dupl -t 100` reports **0 clone groups**. |
| QR-COMPLEX | Cyclomatic complexity | `make complexity`: `cyclop -maxComplexity 10 ./...` PASS. |
| QR-COVER | Production statement coverage | `make coverage`: **>=80% overall**, **>=90% separately for each named critical package**. The existing gate names `internal/authentication` and `internal/authorization`; verify whether all Auth/DPoP/AccessControl code is actually included, as required by QR-08. |
| QR-RACE | Go race detector | `make race`: `go test -race ./...` PASS. |
| QR-INTEGRATION | Integration tests | `make integration`: `go test -tags=integration ./...` PASS; project coverage also uses the `integration` tag. |
| QR-MUTATION | Changed-code mutation testing | `make mutation-pr` with go-gremlins on PR (`make mutation-full` for full run); report killed/lived/not-covered mutations and exact scope. **The approved XML explicitly mandates baseline-first and has no numeric score threshold; do not invent one.** |
| QR-ALL | Aggregated blocking gate | `make quality` includes fmt, vet, lint, security, vulncheck, dupl, complexity, coverage and race; PR CI additionally executes changed-code mutation. |

**Testing evidence:** `go build ./...` and `go test ./...` passing alone do **not** establish `QR-ALL`, integration, mutation adequacy, or release acceptance. Preserve reproducible job URL, commit SHA, measured production coverage by package, mutation outcomes, race/security/lint logs, integration results, and failed or excluded checks. A module-level or staged exception does not waive project thresholds or authorize production release.

**Coverage methodology:** `go test -tags=integration -count=1 -covermode=atomic -coverpkg=./cmd/...,./internal/... -coverprofile=coverage.out ./cmd/... ./internal/...` then `go run ./tools/coverage-gate -profile coverage.out -min 80 -critical-min 90`. Count each unique coverage block once (merge duplicate block hit information); QA and tools are verified separately, not mixed into production coverage. Critical-package policy changes require explicit project approval.

## Issue #25 — mandatory gaps to correct, not exceptions

At the time of this documentation the existing `tools/coverage-gate/main.go` uses a hard-coded production package list that **does not include `internal/issue25`**. Yet `internal/issue25` is production code. Because the profile parser rejects unknown production packages, the current gate cannot validly certify Issue #25 production coverage as-is. This is a **blocking defect in gate scope**, not grounds for excluding code or reducing thresholds.

Before claiming Issue #25 Quality Gate PASS (and separately before claiming full QR/QA/CI/PR compliance):
1. Amend the coverage gate and its tests so **all** production packages, including `internal/issue25`, are counted exactly once; retain the existing **80% overall / 90% named-critical** requirements without silently changing the critical list.
2. Ensure the Issue #25 branch runs the **full existing quality workflow**, not merely the narrower `issue25-validation.yml` workflow (which currently runs build, migrations and `go test ./...`). A separate documented workflow is not a substitute for a required gate.
3. Provide results of mandatory mutation testing, race, integration/real PostgreSQL checks and static/security checks on the exact reviewed SHA.
4. Map approved Issue #25 requirements/contracts to actual automated test assertions and results for `MS25-01` through `MS25-15`; `NOT COVERED` is not PASS.
5. Independently map and verify **all** `QR-01`–`QR-09`, `QA-01`–`QA-10`, `CI-01`–`CI-05` and applicable `PR-01`–`PR-05`, including testcontainers real Auth/DPoP/Centrifuge, Flutter E2E, property/state/model testing, failure/performance, branch protection, mutation baseline and infrastructure isolation. Do not call narrower CI PASS a full Quality Gate PASS.
6. Independent QA reviews evidence before acceptance. No deployment, remote DB changes, or BFE Drive changes are implied by this document.

**Source of authority:** The approved `QualityRequirements.xml` governs. This document records pre-existing repository rules and an observed Issue #25 enforcement gap. It introduces **no new numeric thresholds**. Any future change of policy requires a separately reviewed, explicit Project Owner decision.
