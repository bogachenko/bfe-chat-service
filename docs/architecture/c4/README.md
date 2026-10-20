# BFE Chat Service — C4 Model

**Status:** Draft for independent architecture review. Documentation only; no GRACE approval or runtime changes.

- Source of truth: approved `RequirementsAnalysis.xml`, `Technology.xml`, `DevelopmentPlan.xml`, `Contracts.xml`, and current `cmd/bfe-chat-service/main.go` from `main`.
- C1: system boundaries and user/external-system context.
- C2: Nginx ingress, Go application with embedded Centrifuge, Chat PostgreSQL.
- C3: legacy modules and Issue #25 capabilities within the **same** Go process.
- Distinguish the external BFE Drive system (including Internal Auth via UDS) from the Chat-owned PostgreSQL database.
- C3 is a *logical component view*: several Issue #25 components map to files under the same `internal/issue25` package. It does not claim separate deployed microservices.
- The Nginx ingress reflects the deployed test topology verified during Issue #25 rollout; its deployment assets are not currently merged into `main`.
- Document IDs/titles are metadata snapshots. BFE Drive owns document access decisions.

## Rendering

Open `workspace.dsl` in Structurizr Local, or validate/render with the Structurizr CLI. Diagram keys: `c1-system-context`, `c2-containers`, `c3-chat-components`.

## Review required

Independent QA must validate DSL syntax and each relationship against GRACE + the merged production code. Do not treat this draft as an approved change to GRACE.
