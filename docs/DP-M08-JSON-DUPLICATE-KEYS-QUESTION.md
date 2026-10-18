# DP-M08 — Open GRACE specification question: duplicate JSON keys

Status: **SPECIFICATION QUESTION / HUMAN DECISION REQUIRED**.

Approved `Contracts.xml` MC-08, FC-13 (`HandleHTTP`) and FC-14 (`HandleWebSocket`) require protocol adaptation, delegation, and safe errors; they **do not** define a policy for duplicate keys in JSON request objects. Reviewed `MentalScenarios.xml` and the approved GRACE contracts; no requirement authorizes introducing a new rejection rule.

Example: `{"text":"first","text":"second"}`.

Current Go `encoding/json.Decoder` semantics apply (later duplicate values may replace/merge earlier ones). **This is a description of current implementation behavior, not an approved business requirement.**

Questions for Command Center:
1. Reject duplicate property names with `VALIDATION_ERROR`, or retain standard Go decoder behavior?
2. If rejection is approved, does it apply recursively to nested `document` objects and Centrifuge Connect `data`?
3. Does the same policy apply to all DP-M08 HTTP commands and historical API versions?

Do **not** change JSON policy or declare duplicate-key rejection an acceptance condition before GRACE approval. Client-only DOC-002, DOC-005, DOC-006 remain excluded from server-side acceptance.
