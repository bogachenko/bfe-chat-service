# C4 architecture — GRACE traceability

**Status: DRAFT.** Each diagram is a *view of a specific GRACE artifact*, not an independent architecture decision.

| GRACE stage / source | C4 view | What to verify |
|---|---|---|
| 1. [RequirementsAnalysis.xml](../../../RequirementsAnalysis.xml) | **Requirements → C1** (`requirements-c1-context`) | Actors, external boundaries, use cases: `ACT-USER`, `ACT-EMPLOYEE`, `ACT-ADMIN`, `ACT25-OWNER`, `ACT25-MEMBER`; `REQ25-DIRECT`, `REQ25-GROUP`, `REQ25-SEND`, `REQ25-HISTORY`. |
| 2. [Technology.xml](../../../Technology.xml) + 3. [DevelopmentPlan.xml](../../../DevelopmentPlan.xml) | **Technology/DevelopmentPlan → C2** (`technology-development-c2-containers`) | One Go service, embedded Centrifuge, Chat PostgreSQL, external BFE Drive UDS, HTTPS ingress. |
| 4. [Contracts.xml](../../../Contracts.xml) | **Contracts → C3** (`contracts-c3-components`) | Legacy `MC-01..MC-08` and Issue #25 `MC25-01..MC25-09` implementation responsibilities. |
| 5. [MentalScenarios.xml](../../../MentalScenarios.xml) | **Scenario review** | Verify interactions, privileges, message ordering; dynamic diagrams are not yet modelled. |

## Interpretation

- C1 is the **requirements diagram**: people and logical use cases; arrows from actors to Chat do not claim clients bypass the HTTP ingress.
- C2 is the **technology/deployment architecture**; Nginx is infrastructure for the test runtime (its config is not merged into `main`).
- C3 describes **logical components inside one Go executable**; Issue #25 logic is often co-located in `internal/issue25` rather than deployed separately.
- Current Git main is the source for GRACE and implementation. Document context carries metadata only; actual access to documents is authorized by BFE Drive.
- The diagrams must not silently rewrite legacy GRACE requirements; legacy assignments and Issue #25 self-service chats are separately represented.

## View

Run `make c4` on this branch and open http://localhost:8080.

**Acceptance:** Validate Structurizr DSL, independently trace actor/edge/module claims to GRACE XML and Go, then approve/merge the documentation PR. This model makes no production changes.
