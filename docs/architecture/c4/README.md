# BFE Chat C4 — GRACE views

The approved GRACE XML documents remain the source of truth. This Structurizr workspace is a **visualization**, not a new approval.

| GRACE source | Diagram | Traceability |
|---|---|---|
| `RequirementsAnalysis.xml` | **C1** (`requirements-c1-context`) | `ACT-USER`, `ACT-EMPLOYEE`, `ACT-ADMIN`, `ACT25-USER`, `ACT25-OWNER`, `ACT25-MEMBER`; `REQ25-DIRECT/GROUP/MEMBERSHIP/HISTORY`. Group owner/member are roles of a BFE user, **not additional account categories or direct network clients**. |
| `Technology.xml` and `DevelopmentPlan.xml` | **C2** (`technology-development-c2-containers`) | Single Go process, embedded Centrifuge, PostgreSQL, BFE Drive Internal Auth via UDS, HTTPS ingress. |
| `Contracts.xml` | **C3** (`contracts-c3-components`) | Legacy `MC-01..MC-08`, Issue #25 `MC25-01..MC25-09` as logical responsibilities. |
| `MentalScenarios.xml` | Future dynamic diagrams | Not yet visualized. |

## Correct boundaries

- People → external BFE clients → HTTPS ingress → Chat Go service. **Never** Person → Chat Service directly.
- Chat service → BFE Drive Internal Auth by private UDS. BFE Drive owns documents and their ACL; document ID/title in chat confer no document access.
- PostgreSQL is a **Chat-owned database**, not a new separate software product. The database model does not imply a separate physical VPS.
- `MC25-*` components in C3 are **logical groupings** within `internal/issue25`, not standalone deployed services.
- Nginx test topology is runtime evidence, not a change to the approved requirements.
- Keep arrows short and put requirements/contracts in element descriptions and this traceability table, rather than inventing direct interaction edges to display use cases.

## Run and review

`make c4`; browser: http://localhost:8080. `make c4-stop` stops the viewer.

Independent review must validate DSL using the actual Structurizr parser and trace all system boundaries, module labels and arrows to the approved XML and Go code. **Structurizr parser validation: PASS** ([GitHub Actions #39592100062](https://github.com/bogachenko/bfe-chat-service/actions/runs/39592100062)). **Independent QA approval is still pending.**
