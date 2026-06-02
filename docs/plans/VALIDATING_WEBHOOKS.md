# Plan: Validating admission webhooks (pre–Phase 5)

**Status:** Draft for review — do not implement until approved.  
**Audience:** Future implementer or agent.  
**Schedule:** Complete before [Phase 5 — User & authority management](../ROADMAP.md#phase-5--user--authority-management) (CHLAUTH / AUTHREC CRDs).

---

## 1. Goals & non-goals

### Goals

- Add **validating** admission webhooks (no mutating, no defaulting webhooks) for all **v1alpha1** Kurator CRDs shipped today:
  `QueueManagerConnection`, `Queue`, `Topic`, `Channel`.
- Reject invalid specs **at admission time** with clear `Status().Details` messages, complementing existing OpenAPI/kubebuilder markers (NFR **API-2**).
- Centralize cross-field and referential rules that today only surface after reconcile (missing `connectionRef`, wrong queue type attributes, unsupported channel type if CRD ever widens).
- Keep validation **pure Kubernetes**: `client.Reader` lookups only — **no mqweb / `MQAdmin` calls** in webhooks.
- Extract rules into **`internal/validation`** (or similar) as pure functions; webhooks are thin wrappers — same pattern as thin reconcilers.
- Full test coverage: table-driven unit tests per rule; envtest (or envtest + webhook server) for admission integration.
- Wire **cert-manager** TLS and `ValidatingWebhookConfiguration` in Kustomize **and** Helm; align with existing kind stack (`hack/kind-cluster` already installs cert-manager).

### Non-goals

- **Mutating** webhooks (no defaulting, no label injection, no spec mutation).
- **Conversion** webhooks (single stored version `v1alpha1`).
- Validation that requires talking to IBM MQ (endpoint reachability, MQSC syntax, credential correctness) — stays in `QueueManagerConnection` reconcile / `mqrest` adapter.
- **Phase 5** resources (`Authority`, `CHLAUTH`, users) — add webhooks when those CRDs exist.
- Blocking `QueueManagerConnection` delete with dependents (listed as **optional** below; defer unless cheap).
- CI job dedicated only to webhooks — extend existing `task test:run` / e2e scaffolding.

---

## 2. Current state (baseline)

| Area | Finding |
|------|---------|
| **`cmd/main.go`** | `webhook.NewServer` wired; cert flags (`--webhook-cert-path`, etc.); **no** `SetupWebhookWithManager` / handler registration (`// +kubebuilder:scaffold:builder` empty). |
| **API types** | OpenAPI: required fields, `endpoint` `^https://`, `Queue.type` / `Channel.type` enums, `MinLength` on names. **No** programmatic CEL for `connectionRef` existence or queue-type attribute requirements. |
| **Controllers** | `resolveConnection` + `waitForConnectionReady` on Queue/Topic/Channel; missing QMC → `Synced=False` (not admission deny). `Channel` rejects non-`svrconn` at reconcile via `TerminalError` (OpenAPI enum already limits to `svrconn`). Queue type errors surface from `mqrest.validateQueueType` at MQ call time. **No** check that alias has `targq` or remote has `xmitq`/`rqmname` before define. |
| **Manifests** | `config/default/kustomization.yaml`: `[WEBHOOK]` and `[CERTMANAGER]` sections **commented**; `../webhook` and `../certmanager` **not present** in repo (Kubebuilder stubs never generated). |
| **`config/manager/manager.yaml`** | No webhook container port (9443) or cert volume — patches exist only as commented `manager_webhook_patch.yaml` references. |
| **Helm** | `charts/kurator/templates/deployment.yaml` — no webhook port/certs/Service. |
| **Tests** | `internal/controller/suite_test.go`: envtest with CRDs only, **no** webhook install. E2e: `setupCertManager()` in `test/e2e/e2e_suite_test.go` (for future webhooks); scaffold comments `e2e-webhooks-checks`. |
| **Tooling** | `task manifests` / `hack/verify.sh` already pass `webhook` to `controller-gen`; no webhook YAML until handlers exist. |
| **Kind / cert-manager** | `hack/kind-cluster/terraform/cert-manager.tf` installs cert-manager — **platform ready**, operator **not** wired. |

---

## 3. Priority validation rules

Implement in order below. Shared helpers live in `internal/validation` (package name TBD; keep flat files: `connection.go`, `queue.go`, `names.go`, `attributes.go`).

### 3.1 All workload CRDs (`Queue`, `Topic`, `Channel`)

| Rule | Severity | Implementation notes |
|------|----------|----------------------|
| **`spec.connectionRef.name` non-empty** | Error | Redundant with OpenAPI but cheap. |
| **Referenced `QueueManagerConnection` exists** in **same namespace** | Error | `Get` QMC; `NotFound` → deny. |
| **Referenced QMC not deleting** | Error | `deletionTimestamp != nil` → deny with message to remove dependents first or wait. |
| **Do not require QMC `Ready=True`** | — | Readiness is runtime; admission only checks object existence (avoids mqweb in webhook). |

Use `admission.Warnings` (Kubernetes 1.27+) only where marked “warning” below.

### 3.2 `QueueManagerConnection`

| Rule | Severity | Notes |
|------|----------|-------|
| **`spec.endpoint` HTTPS** | Error | Already OpenAPI `Pattern=^https://`; keep marker; webhook can re-check for defense in depth or rely on OpenAPI only. |
| **`spec.credentialsSecretRef.name` required** | Error | OpenAPI already. |
| **`spec.queueManager` non-empty** | Error | OpenAPI already. |
| **Optional: credentials Secret exists** | Error (recommended) or Warning | `Get` Secret in same namespace. **Recommend Error** for fail-fast UX; requires webhook has same Secret RBAC as reconciler (already `get` on secrets). |
| **Optional: `spec.tls.caSecretRef` Secret exists** when set | Error | Same as above. |
| **Optional: block delete while dependents exist** | Error | On `DELETE`, list Queue/Topic/Channel in namespace with matching `connectionRef`; deny if any exist. **Defer to PR4+** if scope tight — document ownerReferences alternative (not used today). |

### 3.3 `Queue`

| Rule | Severity | Notes |
|------|----------|-------|
| **MQ object name (`spec.queueName`)** | Error | Max **48** chars; charset: `A–Z`, `0–9`, `.`, `/`, `%`, `&`, `$`, `#`, `@` (document subset in validator — match [IBM MQ naming](https://www.ibm.com/docs/en/ibm-mq/9.3.x?topic=reference-mqsc-commands) and `test/integration/mq/objectNameForTest`). Reject leading/trailing `.`, empty after trim, `SYSTEM.*` prefix (reserved). |
| **Type vs required attributes** | Error | After `NormalizeAttrKey` (reuse `mqadmin.NormalizeAttrKey`): **alias** → `targq` required (accept alias `target` → normalized). **remote** → `xmitq` and `rqmname` required (accept `transmissionqueue`, `remotemanager`). **local** → no extra required keys. |
| **Unknown attribute keys** | **Warning** (optional) | Union of keys from `queueLocalDisplayParameters`, `queueAliasDisplayParameters`, `queueRemoteDisplayParameters`, plus documented passthrough keys in [ATTRIBUTE_RECONCILIATION.md](../ATTRIBUTE_RECONCILIATION.md) (`maxmsglen`, trigger fields, etc.). Unknown key → `admission.Warnings` with “ignored for drift / may fail at MQ”. **Do not error** on unknown keys (GitOps forward-compat). |
| **Channel type** | N/A | — |

Align error messages with reconcile `TerminalError` text where applicable so users see consistent wording.

### 3.4 `Topic`

| Rule | Severity | Notes |
|------|----------|-------|
| **`spec.topicName`** | Error | Same MQ name rules as queue (48 chars, charset, no `SYSTEM.*`). |
| **Unknown attribute keys** | Warning (optional) | Union from `topicDisplayParameters` + passthrough in ATTRIBUTE_RECONCILIATION. |

### 3.5 `Channel`

| Rule | Severity | Notes |
|------|----------|-------|
| **`spec.channelName`** | Error | MQ channel name rules (same charset; 48 char limit). |
| **`spec.type` must be `svrconn`** (or empty → default) | Error | OpenAPI enum; webhook rejects explicit future values if CRD widens before webhook ships. |
| **Unknown attribute keys** | Warning (optional) | Union from `channelDisplayParameters` + passthrough (e.g. `sslciph`). |

### 3.6 Explicitly out of scope for webhooks

- Secret key presence (`username` / `password`) — remains reconcile-time in `mqrest` factory.
- Attribute value domains (numeric ranges, enum values for `targtype`) — MQ returns terminal errors; optional later.
- Duplicate MQ object names across CRs — allowed (two CRs could target same QM object); no webhook.

---

## 4. Kubebuilder implementation steps

### 4.1 Generate webhook scaffolding (per kind)

From repo root (Kubebuilder v4 / `PROJECT` layout):

```bash
kubebuilder create webhook \
  --group messaging \
  --version v1alpha1 \
  --kind QueueManagerConnection \
  --programmatic-validation \
  --force

# Repeat for Queue, Topic, Channel
```

Expected artifacts (paths may match scaffold defaults):

| Artifact | Purpose |
|----------|---------|
| `internal/webhook/v1alpha1/*_webhook.go` | `CustomValidator` impl: `ValidateCreate`, `ValidateUpdate`, `ValidateDelete` |
| `config/webhook/manifests.yaml` | `ValidatingWebhookConfiguration` |
| `config/webhook/service.yaml` | Service `:443` → pod `9443` |
| `config/default/manager_webhook_patch.yaml` | Container port + cert volume mount |
| `config/certmanager/certificate.yaml`, `issuer.yaml` | cert-manager `Certificate` for serving cert |
| `config/crd/kustomizeconfig.yaml` | CA injection for CRDs (if conversion later; optional for validating-only) |

Add `+kubebuilder:webhook` markers on each validator (example for Queue):

```go
// +kubebuilder:webhook:path=/validate-messaging-kurator-dev-v1alpha1-queue,mutating=false,failurePolicy=fail,sideEffects=None,groups=messaging.kurator.dev,resources=queues,verbs=create;update,versions=v1alpha1,name=vqueue.kb.io,admissionReviewVersions=v1
```

Use **distinct** `name=` per resource; `failurePolicy=fail` (see §7).

### 4.2 Register in `cmd/main.go`

After reconciler setup, before health checks:

```go
if err := messagingwebhook.SetupWithManager(mgr); err != nil {
    setupLog.Error(err, "unable to setup webhooks")
    os.Exit(1)
}
```

`SetupWithManager` registers all validators on the manager’s webhook server (pattern from kubebuilder book).

### 4.3 Enable Kustomize overlays

Uncomment in `config/default/kustomization.yaml`:

- `resources`: `../webhook`, `../certmanager`
- `patches`: `manager_webhook_patch.yaml`
- `replacements`: cert-manager CA injection blocks for `ValidatingWebhookConfiguration` (and metrics if desired later)

Ensure `config/crd/kustomization.yaml` webhook cert injection comments are updated only if needed (validating webhooks do not require CRD conversion CA).

### 4.4 cert-manager / TLS

- **Issuer**: `selfSigned` in operator namespace (scaffold default) — sufficient for in-cluster webhooks.
- **Certificate**: DNS names `$(SERVICE_NAME).$(NAMESPACE).svc` and cluster-local variant (follow scaffold `replacements` in `kustomization.yaml`).
- **Deployment**: mount Secret at `/tmp/k8s-webhook-server/serving-certs` (scaffold path); set `--webhook-cert-path` in `manager_webhook_patch.yaml` to match `cmd/main.go` flags.
- **Helm**: add optional `webhooks.enabled`, `Certificate` / issuer templates or document cert-manager as prerequisite; mount same path; Service `kurator-webhook-service` on 443.

**Blocker today:** `config/webhook/` and `config/certmanager/` **do not exist** — first PR must add scaffold output and uncomment kustomize. cert-manager is on kind but **not** referenced by operator manifests.

### 4.5 RBAC

`controller-gen` emits webhook RBAC (`+/webhook` markers or `rbac` for `validatingwebhookconfigurations` read if needed). Verify `config/rbac/role.yaml` includes:

- `get`/`list`/`watch` on `queuemanagerconnections`, `queues`, `topics`, `channels` (for connectionRef + delete protection).
- `get` on `secrets` (if Secret existence checks enabled).

Run `task manifests` and `task verify`.

### 4.6 `failurePolicy`, timeouts, selectors

| Setting | Recommendation |
|---------|----------------|
| **failurePolicy** | `Fail` — invalid CRs must not persist; cluster without working webhook blocks creates (acceptable for this operator). Document `Ignore` only for emergency ops. |
| **timeoutSeconds** | `10` (default); keep webhook logic O(1) API calls. |
| **namespaceSelector** | **None** (validate in all namespaces) unless product later needs opt-in. |
| **objectSelector** | **None** |
| **matchPolicy** | `Equivalent` (default) |
| **sideEffects** | `None` |

### 4.7 Leader election vs webhooks

- Reconcilers: leader-elected (`--leader-elect`).
- Webhooks: served on **all** replicas (controller-runtime default) — correct for HA.

### 4.8 Reconcile behavior after webhooks

- Keep existing reconcile checks for **defense in depth** (race: QMC deleted between admission and reconcile) — do not remove `resolveConnection` / `waitForConnectionReady`.
- Optionally narrow reconcile error messages when webhook already guarantees preconditions.

---

## 5. Testing strategy

| Layer | What | How | Mock vs real |
|-------|------|-----|----------------|
| **Unit** | `internal/validation/*` pure functions | `testing` table tests: names, type/attr matrix, connection ref cases | No API server |
| **Unit** | Webhook adapters | Test `ValidateCreate` with `fake.Client` pre-seeded QMC/Secret | fake client |
| **Envtest admission** | End-to-end deny/allow via API server | Extend `internal/controller/suite_test.go` **or** new `internal/webhook/v1alpha1/suite_test.go`: `envtest.Environment{ WebhookInstallOptions: ... }`, install `ValidatingWebhookConfiguration` from `config/webhook/manifests.yaml`, run manager webhook in-process with `envtest` (controller-runtime pattern) | Real apiserver + real webhook config; **mock** MQ not needed |
| **Envtest negative cases** | Apply invalid Queue without `targq` → expect `Forbidden` / `Invalid` | `Expect(Create).To(MatchError(...))` | — |
| **Envtest warnings** | Unknown attr → create succeeds + warning in audit (if asserted) | Optional; K8s version in envtest must support admission warnings | — |
| **Existing controller envtest** | No regression | Current reconciler tests unchanged | — |
| **E2e** | One scenario: invalid CR rejected by API | `kubectl apply` invalid manifest → non-zero exit; valid samples still work | Real cluster + cert-manager (already installed on kind) |
| **CI** | `task test:run` | Add webhook suite to Ginkgo/Makefile if split; envtest startup +2–5s | No new workflow required initially |
| **Coverage** | `internal/validation` | Target **≥90%** on new package; keeps `internal/` ≥85% gate |

### envtest webhook install sketch

```go
testEnv := &envtest.Environment{
    CRDDirectoryPaths:     []string{...},
    WebhookInstallOptions: envtest.WebhookInstallOptions{
        Paths: []string{filepath.Join("config", "webhook", "manifests.yaml")},
    },
}
// Start envtest, start mgr with webhook only or full mgr, run admission tests
```

Use self-signed certs from envtest or cert-manager test issuer; follow controller-runtime envtest webhook docs.

### What not to test in webhooks

- mqweb ping, MQSC errors, drift — covered by adapter/integration/e2e tests.

---

## 6. Phased rollout (PR plan)

| PR | Scope | Delivers |
|----|--------|----------|
| **PR1 — Infra + certs** | `kubebuilder create webhook` for all kinds; `config/webhook`, `config/certmanager`; uncomment kustomize; `main.go` registration; Deployment patch (port 9443, cert volume); Helm Service + cert volumes; RBAC; **no** business rules beyond `noop`/always allow | Webhook pod serves TLS; VWC registered; e2e cert-manager path exercised; envtest webhook install smoke test |
| **PR2 — QMC + Queue** | `internal/validation`; QMC Secret optional checks; Queue name + type/attr rules; unit + envtest denials | Highest-value rules for misconfiguration |
| **PR3 — Topic + Channel** | Name rules; channel type; optional attribute warnings; envtest + one e2e negative test | Parity across object types |
| **PR4 — Docs (post-approval)** | Update `docs/ROADMAP.md`, `AGENTS.md`, `ARCHITECTURE.md`, `INSTALL_AND_USE.md`, `NON_FUNCTIONAL_REQUIREMENTS.md` (API-2 verification), `docs/CICD.md` if needed; optional QMC delete protection | User-facing contract |

Each PR: `task verify`, `task lint`, `task test:run`, relevant e2e if manifests change.

---

## 7. Risks & operations

| Risk | Mitigation |
|------|------------|
| **Webhook unavailable** | With `failurePolicy: Fail`, API server rejects CR creates/updates — operator appears “down” for writes. Run **≥2** replicas + PDB; monitor webhook latency; cert-manager `Certificate` Ready. |
| **Cert rotation** | cert-manager rotates; controller-runtime cert watcher reloads from mount — verify on upgrade test. |
| **HA / multiple replicas** | All replicas serve webhooks; enable leader election for controllers only (current). |
| **Latency** | Only same-namespace GET/LIST; no external calls; timeout 10s. |
| **Race: QMC deleted after admission** | Reconciler still handles missing connection. |
| **Helm without cert-manager** | Document hard requirement or ship cert-manager subchart toggle `webhooks.certManager.create`. |
| **API version skew** | `admissionReviewVersions: v1` only. |

**failurePolicy: Fail vs Ignore**

- **Fail (chosen):** Safer for a personal GitOps operator — never silently accept bad specs.
- **Ignore:** Use only if webhook TLS breaks production; document as break-glass.

---

## 8. ROADMAP / AGENTS.md draft bullets (paste after approval)

Do **not** commit these until the plan is approved.

### `docs/ROADMAP.md` — insert after Phase 4 exit criteria, before Phase 5

```markdown
## Phase 4b — Validating admission webhooks (pre–Phase 5)

- [ ] Validating webhooks (no mutating) for `QueueManagerConnection`, `Queue`, `Topic`, `Channel`.
- [ ] cert-manager TLS for webhook serving; Kustomize + Helm wired.
- [ ] Referential checks: `connectionRef` exists, same namespace, not deleting.
- [ ] Queue: MQ name constraints; alias/remote required attributes; optional unknown-attribute warnings.
- [ ] Topic/Channel: name constraints; channel `svrconn` only.
- [ ] Unit tests (`internal/validation`) + envtest admission tests; optional e2e negative apply.
- [ ] Optional: deny `QueueManagerConnection` delete while dependent CRs exist.

Exit criteria: invalid sample manifests rejected by `kubectl apply`; `task test:run` includes webhook admission tests; kind/Helm install enables webhooks by default.
```

### `AGENTS.md` — Architecture / testing snippets

```markdown
<!-- In Architecture summary diagram or Components table, add: -->
- Validating admission webhooks (`internal/webhook`, `internal/validation`) — reject invalid CR specs before reconcile; no mqweb calls.

<!-- In Testing strategy bullet list, add: -->
- **Admission**: envtest installs `ValidatingWebhookConfiguration`; table-driven tests for `internal/validation`; no MQ.

<!-- In Task table (optional row): -->
| `task test:admission` | Run webhook/envtest admission suite only (if split from test:run) |
```

### `docs/NON_FUNCTIONAL_REQUIREMENTS.md` — API-2 verification column

Change verification for API-2 from “envtest” to “OpenAPI + validating webhooks; envtest admission tests”.

---

## 9. Acceptance criteria (done before Phase 5)

- [ ] `ValidatingWebhookConfiguration` deployed by default (`task deploy` / Helm `webhooks.enabled=true`).
- [ ] cert-manager `Certificate` Ready; pod mounts serving cert; no TLS errors in manager logs on webhook requests.
- [ ] All four CR kinds have validating webhooks registered and `failurePolicy: Fail`.
- [ ] Rules in §3 implemented (optional items documented if deferred).
- [ ] `internal/validation` covered by table-driven unit tests.
- [ ] Envtest proves at least: (1) Queue with missing `connectionRef` target denied, (2) alias Queue without `targq` denied, (3) valid sample allowed.
- [ ] `task verify`, `task lint`, `task test:run` green; `internal/` coverage ≥85%.
- [ ] One e2e or documented manual check: invalid manifest fails `kubectl apply`.
- [ ] No mutating webhooks; no mqweb in admission path.
- [ ] `docs/ROADMAP.md` / `AGENTS.md` updated (PR4) after plan approval.

---

## 10. Reference links

- [Kubebuilder — Admission webhooks](https://book.kubebuilder.io/cronjob-tutorial/webhook-implementation.html)
- [controller-runtime webhook](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/webhook)
- [ATTRIBUTE_RECONCILIATION.md](../ATTRIBUTE_RECONCILIATION.md) — attribute allow-lists for warnings
- [ARCHITECTURE.md](../ARCHITECTURE.md) — thin reconcilers, error classes
- [PHASE5_AUTH_SKETCH.md](../PHASE5_AUTH_SKETCH.md) — out of scope until Phase 5 CRDs exist

---

## Appendix A — Suggested file layout

```
internal/
  validation/
    names.go
    names_test.go
    connection.go
    connection_test.go
    queue.go
    queue_test.go
    topic.go
    channel.go
    attributes.go
    attributes_test.go
  webhook/
    v1alpha1/
      setup.go
      queuemanagerconnection_webhook.go
      queue_webhook.go
      topic_webhook.go
      channel_webhook.go
      suite_test.go          # envtest admission
config/
  webhook/
  certmanager/
  default/
    manager_webhook_patch.yaml
```

---

## Appendix B — First PR checklist (infra only)

1. Run `kubebuilder create webhook` for all four kinds.
2. Uncomment `[WEBHOOK]` / `[CERTMANAGER]` in `config/default/kustomization.yaml`.
3. Register webhooks in `cmd/main.go`.
4. `task manifests && task verify`.
5. Deploy on kind; `kubectl get validatingwebhookconfiguration`; check manager logs for “Serving webhook server”.
6. Add envtest smoke: valid Queue create still works with webhooks installed (validators temporarily allow all).
