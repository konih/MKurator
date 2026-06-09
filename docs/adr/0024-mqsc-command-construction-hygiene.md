# ADR-0024: MQSC command construction hygiene (structured-first, validated strings)

- **Status**: Accepted
- **Date**: 2026-06-09
- **Extends**: [ADR-0002](0002-manage-mq-via-mqweb-rest.md) (mqweb REST transport)

## Context

[ADR-0002](0002-manage-mq-via-mqweb-rest.md) standardised on the mqweb `/mqsc`
endpoint. That endpoint accepts two request shapes:

- **`runCommandJSON`** — structured `command`/`qualifier`/`name`/`parameters`;
  mqweb assembles the MQSC. Queue, Topic, and Channel DEFINE/DISPLAY/DELETE
  already use this exclusively (`internal/adapter/mqrest/client.go`).
- **`runCommand`** — a raw MQSC string. Used today by the auth paths
  (`SET CHLAUTH` / `SET AUTHREC` / their DISPLAYs in
  `internal/adapter/mqrest/auth.go`) and by the `RunMQSC` test-fixture helper.

The 2026-06-09 reliability audit found **MQSC injection** through the raw
string path (EC-P1-04): `ChannelAuthRule.spec.userSource`/`checkClient` and
`AuthorityRecord.spec.authorities[]` are interpolated **unquoted** into
`SET …` strings with no enum or charset validation — a CR author can append
arbitrary MQSC keywords (e.g. `usersrc: "MAP) MCAUSER('mqm'"`) and escalate
privileges on the queue manager. A second concern: per-mqweb-version DISPLAY
safe lists (`mqsc_params.go`) are hardcoded and must be hand-maintained as
versions diverge ([ADR-0010](0010-drift-based-mq-reconciliation.md)).

## Decision

1. **Structured-first.** Every `Admin` port operation uses `runCommandJSON`
   wherever mqweb supports the command. The auth paths are migrated to
   `runCommandJSON` if SET CHLAUTH/AUTHREC prove expressible there (to be
   verified against the live mqweb schema in the Docker integration tier);
   raw `runCommand` remains only where mqweb offers no structured equivalent.
2. **No unvalidated user input in command strings.** For any path that must
   remain string MQSC:
   - Every CR-author-controlled value interpolated into a command is either
     (a) constrained to a strict enum/pattern at the CRD (OpenAPI/CEL) **and**
     re-checked in the webhook, or (b) quoted/escaped by a single shared
     MQSC-quoting helper in `mqrest` (single quotes doubled, parentheses and
     control characters rejected). Defense in depth: both where possible.
   - Concretely (EC-P1-04): `userSource`, `checkClient` become CRD enums
     (their MQSC value sets are closed); `authorities[]` entries get a
     `^[A-Za-z0-9+_]+$`-class pattern matching valid MQ authority keywords.
3. **`RunMQSC` stays fixture-only.** The raw helper remains excluded from
   reconciler reach (enforced by arch-lint per [GO_MODULE.md](../GO_MODULE.md));
   this ADR reaffirms that boundary.
4. **Capability probing (direction, not commitment).** Rather than growing
   per-version DISPLAY lists indefinitely, the adapter may probe attribute
   displayability once per connection at QMC Ready time and cache the result
   on the QMC status. This subsumes the "probe DISPLAY for DEFINE-only queue
   attrs" backlog item; implementation is a Phase 7 roadmap entry and may be
   dropped if mqweb version drift stays manageable.

## Consequences

- Closes the only known privilege-escalation path from CR authorship to
  queue-manager admin (EC-P1-04); admission rejects malformed values before
  any MQ call ([ADR-0009](0009-validating-admission-webhooks.md) unchanged).
- Webhook envtest (audit T6) and adapter unit tests must pin the injection
  attempts; the Docker integration tier verifies the structured auth
  commands against live mqweb before migration lands.
- Tightening `userSource`/`checkClient` to enums is a **validation-tightening
  change**: previously-accepted (malformed) specs become invalid. Acceptable
  at `v1alpha1`; release notes must flag it.
- The quoting helper centralises a concern currently scattered across
  `auth.go` format strings.
- Capability probing, if built, removes a class of hand-maintained tables but
  adds one-time probe latency to QMC readiness — measured before adoption.

## Alternatives considered

- **Quote-only (no enums)**: escaping alone protects MQ but still lets
  garbage values travel to MQ and fail late; enums give users immediate
  admission feedback. Rejected as sole measure.
- **Validate-only (no quoting)**: leaves the adapter unsafe by construction
  for any future field that misses validation. Rejected as sole measure.
- **Ban raw `runCommand` outright**: not possible until structured SET
  CHLAUTH/AUTHREC support is confirmed; fixtures legitimately need it.

## References

- [ADR-0002](0002-manage-mq-via-mqweb-rest.md) — transport (extended)
- [ADR-0014](0014-mq-error-taxonomy-and-requeue.md) — error taxonomy at the adapter
- Edge-case audit 2026-06-09: EC-P1-04, EC-P2-05 (internal)
- [docs/schemas/](../schemas/) — mqsc-runcommand JSON schema
