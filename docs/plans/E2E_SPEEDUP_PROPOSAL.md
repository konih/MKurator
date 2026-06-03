# E2E speedup proposal

**Status:** Phases A–C implemented (2026-06-03). Image cache partially started in CI actions; see Phase C item 5.  
**Date:** 2026-06-03  
**Scope:** `test/e2e/`, `.github/workflows/e2e.yaml`, `hack/ci/*`, `task ci:e2e`.

---

## Executive summary

Today’s e2e path spends most wall-clock time **outside** individual assertions: **kind + Terraform + IBM MQ** per CI job, **repeated image builds and operator deploys**, and **`waitForControllerAndWebhookReady`** on nearly every MQ spec. The Ginkgo suite is **fully serial** (`Serial` on both top-level `Describe`s; `Ordered` inside MQ contexts) and shares **one Kubernetes namespace** (`kurator-system`) and **fixed MQ object names** on a **single QM1**.

A phased plan can cut **PR e2e job** time by roughly **40–55%** (Phase A) and **suite-only** time on an existing cluster by **50–75%** (Phase B), enabling roughly **2×–4×** more MQ lifecycle scenarios within the current **90-minute** workflow budget—after stabilizing known failures (metrics readiness, Helm deploy) noted in [DELTA_AUDIT_2026-06-03.md](./DELTA_AUDIT_2026-06-03.md).

**Top recommendations (detail in [Recommended phased plan](#recommended-phased-plan)):**

1. **Phase A:** One operator deploy for the whole run; hoist MQSC fixture to `BeforeSuite`; cache webhook readiness; drop duplicate `task deploy` / image builds.
2. **Phase B:** Parallel Ginkgo nodes with **per-family K8s namespaces** and **unique MQ object prefixes** on the same QM1; keep CHLAUTH specs on a dedicated serial lane for `DEV.APP.SVRCONN.0TLS`.
3. **Phase C:** PR runs **kustomize only**; Helm e2e on `main` / schedule; shift adapter-heavy cases to **Docker integration**; e2e = operator + CR lifecycle smoke.

---

## Current architecture

### CI and local entry points

| Step | Where | Notes |
|------|--------|------|
| Platform | `task cluster:up` | kind → mkcert → Terraform (ingress, cert-manager, monitoring, IBM MQ). First apply **5–15 min** ([`hack/kind-cluster/Taskfile.yml`](../../hack/kind-cluster/Taskfile.yml)). |
| MQ readiness | `hack/ci/wait-mqweb.sh` | Up to **60 × 10s** (~10 min cap); typical **2–5 min** once MQ pod is up. |
| Suite | `task test:e2e` → `hack/ci/run-e2e.sh` | `go test -tags=e2e ./test/e2e/... -race -timeout=90m`. Acquires `exclusive-test.lock`. |
| Full parity | `task ci:e2e` | `run-ci-e2e.sh`: cluster:up + wait + test:e2e (lock held). |

[`.github/workflows/e2e.yaml`](../../.github/workflows/e2e.yaml):

- **`e2e (kustomize)`:** Every qualifying PR + `main` push — full platform + suite.
- **`e2e (helm)`:** `main` push + `workflow_dispatch` only — **second full `cluster:up`** on another runner.
- Both jobs: **90 min** timeout, `CERT_MANAGER_INSTALL_SKIP=true` (cert-manager from Terraform).

Concurrency group `e2e-${{ github.ref }}` queues on `main`; PR runs cancel in-flight (saves duplicate spins only when superseded).

### Ginkgo structure (today)

```
TestE2E (single process, no -nodes)
├── BeforeSuite: docker:build manager, kind load, curl image build/load, cert-manager check
├── Describe("Manager", Serial, Ordered)
│   ├── BeforeAll: namespace + deployOperatorForE2E (task deploy)
│   ├── 4 × It (pod, webhook×2, metrics up to 5m Eventually)
│   └── AfterAll: undeploy + delete kurator-system
└── Describe("Post-manager IBM MQ integration", Serial, Label("mq"))  [KURATOR_E2E_MQ=1]
    ├── Context fixture: MQSC channel-auth-prereq (2m Eventually)
    ├── Context("Queue reconciliation", Ordered)
    │   ├── BeforeAll: ensureOperatorForMQE2E (re-deploy)
    │   ├── BeforeEach: waitForControllerAndWebhookReady + mq-credentials
    │   └── 5 × It (queue CRUD, attr update, QMC rotation, topic, channel)
    └── Context("Auth reconciliation", Ordered)
        ├── BeforeAll: ensureOperatorForMQE2E (re-deploy again)
        ├── BeforeEach: waitForControllerAndWebhookReady + secret
        └── 3 × It (CHLAUTH ADDRESSMAP, BLOCKUSER, AUTHREC) — each reapplies fixture
```

**Spec count:** 12 `It`s (4 Manager + 8 MQ). No `ginkgo -nodes`, no label filtering in CI.

### Shared resources

| Resource | Value | Implication |
|----------|--------|-------------|
| K8s namespace | `kurator-system` | All CRs and operator; Manager `AfterAll` deletes it — MQ suite must redeploy. |
| QMC name | `e2e-qm1` | Same connection CR name in every spec. |
| MQ queue/topic/channel | `E2E.APP.ORDERS`, `E2E.RETAIL.ORDERS`, `E2E.ORDERS.APP` | Reused across specs; safe only while **serial**. |
| CHLAUTH channel | `DEV.APP.SVRCONN.0TLS` (`e2eChannelName`) | Fixture + all auth specs; **global on QM1** — parallel specs must not fight on this channel. |
| mqweb (operator) | `https://ibm-mq.ibm-mq.svc:9443` | In-cluster; tests also hit NodePort via `KURATOR_E2E_MQ_*` for direct mqrest assertions. |

---

## Timeline estimate (code-derived, not a fresh run)

Observed reference: local `task ci:e2e` **~33 min** before Manager metrics failure ([DELTA_AUDIT_2026-06-03.md](./DELTA_AUDIT_2026-06-03.md)). Below is a **structural** model for a **green** run on `ubuntu-latest` (cold IBM MQ image pull).

| Phase | Low | Typical | High | Dominant factors |
|-------|-----|---------|------|------------------|
| `cluster:up` | 8 min | 12 min | 20 min | kind, Terraform, MQ Helm, image pull |
| `wait-mqweb` | 1 min | 3 min | 10 min | `MQWEB_WAIT_*` cap |
| BeforeSuite images | 2 min | 4 min | 8 min | `docker:build` ×2, kind load ×2 |
| Manager `Describe` | 5 min | 12 min | 20 min | `task deploy`, cert wait (5m), metrics curl (5m Eventually) |
| MQ `Describe` | 15 min | 28 min | 45 min | 2× `ensureOperatorForMQE2E`, 8× `waitForControllerAndWebhookReady`, 3× fixture, 3m/2m Eventually per spec |
| **Suite subtotal** (`test:e2e`) | **22 min** | **44 min** | **73 min** | |
| **Full CI job** (`cluster:up` + wait + suite) | **32 min** | **59 min** | **103 min** | Second job duplicates platform for Helm |

**Per-spec order of magnitude (MQ, warm operator):** ~2–6 min each (QMC apply + Synced Eventually up to 3m + MQ GET + delete wait up to 2m). Auth specs add channel sync + duplicate fixture (~+1–2 min).

**Helm vs Kustomize:** Same suite; Helm `BeforeAll` deletes namespace first (`deploy_helpers.go`). Extra **1–3 min** per deploy path; does not justify a second full platform spin on every PR.

---

## Bottleneck analysis

### 1. Platform cost (once per job, not amortized across specs)

- CI runs **`cluster:up` + `cluster:down`** per job; no cross-job cluster reuse.
- **Two jobs on `main`** (kustomize + helm) ≈ **2× platform** cost (~24 min typical × 2).

### 2. Serial Ginkgo execution

- `Describe(..., Serial)` on Manager and MQ prevents any intra-suite parallelism.
- `Ordered` inside MQ contexts forces sequence even when object names could be isolated.

### 3. Repeated operator lifecycle

- Manager **`AfterAll`** undeploys and deletes `kurator-system`.
- **`ensureOperatorForMQE2E`** runs in **both** Queue and Auth `BeforeAll` → **two full redeploys** after Manager teardown.
- Each deploy invokes **`task deploy` / `task deploy:helm`** → **`docker:build` again** (BeforeSuite already built and loaded the image).

### 4. `waitForControllerAndWebhookReady` amplification

Called from:

- Every `deployOperatorForE2E` / `ensureOperatorForMQE2E` (up to **3×** per run).
- **Every MQ `BeforeEach`** (8 specs) — each can wait up to **~5m** certificate + rollout + webhook probe.

Even at **~1 min** per successful call, **8 BeforeEach × 1 min ≈ 8 min** of mostly redundant waiting.

### 5. MQSC fixture duplication

- `channel-auth-prereq.mqsc` applied in: dedicated fixture `It`, both CHLAUTH specs (**3×** per run).
- Fixture `It` is largely redundant if auth `BeforeSuite` applies once (idempotent `REPLACE`).

### 6. Polling and delete waits

- Synced/Ready: **3 min** `Eventually` × many specs.
- CR delete: **`kubectlWaitTimeout` 2m** + MQ `Eventually` 2m per delete path.
- Metrics spec: **5 min** curl pod + **3 min** log substring.

### 7. What must stay serial (today)

| Constraint | Reason |
|------------|--------|
| Same `mqQueueObject` / CR names | Constants in `mq_e2e_test.go`; parallel specs would collide on QM1 and in one namespace. |
| `DEV.APP.SVRCONN.0TLS` + CHLAUTH | Single SVRCONN; ADDRESSMAP/BLOCKUSER rules keyed by channel name — concurrent SET/DELETE races. |
| `mq-credentials` secret name | Shared; QMC rotation spec mutates password — must not overlap other specs. |
| Manager metrics + webhook tests | Assume single controller deployment in `kurator-system`. |
| Machine lock | Only one of e2e / integration / ci:e2e per host — parallel **local** jobs unsafe; CI uses separate runners per job. |

### 8. Helm vs Kustomize duplication

- Not duplicate **within** one `go test` invocation — env `KURATOR_E2E_DEPLOY` selects one path.
- **CI duplication** is **two workflows jobs** each doing full platform + full suite (~2× wall clock on `main`).

---

## Options (tradeoffs)

| # | Option | Est. savings | Pros | Cons / risks |
|---|--------|--------------|------|----------------|
| 1 | **Parallel Ginkgo nodes** + namespaces (`e2e-queues`, `e2e-topics`, `e2e-auth`) | Suite **40–60%** with 3–4 nodes | Same QM1; scales spec count | MQ name discipline; CHLAUTH lane must stay serial; `-race` + parallel kubectl load; flakier webhook timing |
| 2 | **Single deploy, parallel specs** with unique MQ names per `It` | Suite **25–40%** | Simpler than multi-node; fewer redeploys | Still one process unless nodes added; must generate unique `queueName`/CR names |
| 3 | **Split CI by label** (`mq-queue`, `mq-auth`) sharing one cluster | CI calendar **−0%** per PR unless platform shared | Targeted failures | **One runner cannot** run two locked suites; needs one job orchestrating ginkgo labels or self-hosted pool |
| 4 | **Fixture MQSC once** in `BeforeSuite` / Auth `BeforeAll` | **2–5 min** | Easy win | Fixture drift if channel deleted by another spec; keep idempotent `REPLACE` |
| 5 | **Skip or slim Manager suite** on `KURATOR_E2E_MQ=1` CI | **5–15 min** | MQ-focused PR signal | Loses metrics/webhook e2e unless moved to envtest/smoke |
| 6 | **Helm job only on schedule**; kustomize on PR | **~50% platform** on PR; **`main` still ~2×** if both run | Matches [e2e.yaml](../../.github/workflows/e2e.yaml) partial intent | Helm path less frequent regression signal |
| 7 | **Docker integration** for adapter; kind e2e smoke only | Suite **50–70%** | Fast PR feedback; integration already has CHLAUTH/AUTHREC | Loses “operator + in-cluster mqweb” for moved scenarios |
| 8 | **One cluster, two deploy modes** (kustomize + helm namespaces) | **`main` platform −50%** | One `cluster:up`, two operator installs | Two controllers webhooks — need distinct release names / webhook configs; higher complexity |
| 9 | **QMC + secret per namespace** | Enables #1/#2 | Isolates credential rotation tests | More YAML boilerplate; still one QM1 |

---

## Recommended phased plan

### Phase A — Quick wins (low risk, 1–2 PRs)

**Goal:** ~**15–25 min** off suite time; no Ginkgo parallelism yet.

1. **Keep operator up for the full run**  
   - Remove Manager `AfterAll` namespace delete **or** move MQ specs before Manager and delete once at end.  
   - Drop redundant **`ensureOperatorForMQE2E`** in Auth `BeforeAll` if Queue context already deployed (single `BeforeAll` on parent `Describe`).

2. **Deploy once, reuse image**  
   - BeforeSuite: build + load image only.  
   - `deployOperatorForE2E`: use `task deploy:operator` + `install:crds` (no `docker:build` dep) — mirror split already in [`Taskfile.yml`](../../Taskfile.yml).

3. **Webhook readiness cache**  
   - Package-level `sync.Once` or Ginkgo `BeforeAll` on MQ `Describe`: call `waitForControllerAndWebhookReady` once; `BeforeEach` only refresh on prior failure / rollout.

4. **MQSC fixture once**  
   - Apply `channel-auth-prereq.mqsc` in MQ `BeforeAll`; remove standalone fixture `It` or reduce to a cheap GET check.

5. **Tighten Eventually where safe**  
   - After stable CI green: Synced **3m → 90s** for known-fast paths; keep 3m for QMC rotation only.

6. **CI doc alignment**  
   - [DEVELOPMENT.md](../DEVELOPMENT.md) still says Helm e2e “not in CI”; workflow **does** run `e2e-helm` on `main` — update when changing schedule.

**Estimated impact:** Suite **~44 min → ~25–32 min** typical; full job **~59 min → ~40–48 min**.

### Phase B — Parallel namespaces (medium risk, 2–4 PRs)

**Goal:** Suite **~50–65%** faster on warm cluster; headroom for new specs.

1. **Enable `ginkgo -nodes=N`** (start N=3) in `run-e2e.sh`; document `-race` interaction (may need `CGO_ENABLED=1` and fewer nodes on small runners).

2. **Namespace per family** (same operator cluster-scoped RBAC):  
   - `kurator-e2e-queues`, `kurator-e2e-topics`, `kurator-e2e-channels`, `kurator-e2e-auth`  
   - Each: own `mq-credentials`, `e2e-qm1` QMC (same endpoint/QMGR), **unique object name prefix** (`E2E.Q1.ORDERS`, …).

3. **Labels and lanes**  
   - `Label("mq-queue")`, `Label("mq-auth-serial")`  
   - Run auth + CHLAUTH on **one node** (`ginkgo --procs=1` for label `mq-auth-serial` or dedicated subprocess).

4. **CHLAUTH isolation**  
   - Option A: keep `DEV.APP.SVRCONN.0TLS` only in serial auth lane.  
   - Option B: per-spec channel `E2E.CH.<uuid>.TLS` + fixture template parameterized (more work, true parallelism).

5. **Parameterized manifests**  
   - Helper `mqObjectPrefix(nodeID string)` to avoid copy-paste and collisions.

**Estimated impact:** MQ portion **~28 min → ~10–14 min**; full suite **~25 min** on existing cluster (excl. platform).

### Phase C — CI and pyramid restructuring ✅

**Goal:** PR signal in **<25 min** platform+suite; deeper coverage without 90m creep.

1. **PR `e2e.yaml`:** kustomize job only; **Helm on `workflow_dispatch` + weekly cron** — done.

2. **`main`:** **single `e2e` job** — kustomize suite then `task test:e2e:helm` on same cluster — done.

3. **Tier redistribution** ([ADR-0011](../adr/0011-layered-testing-strategy.md)) — done:  
   - **Integration:** alias/remote queue, replace semantics, CHLAUTH/AUTHREC edge cases.  
   - **E2e trimmed:** removed queue attribute-update and BLOCKUSER CHLAUTH specs. Kept: local queue/topic/channel/ADDRESSMAP CHLAUTH/AUTHREC happy paths + delete, fixture smoke, slow QMC rotation, Manager smoke + webhook denials.

4. **Label filter in CI:** PR `(smoke || mq) && !slow`; full suite on `main` (kustomize + helm steps) and cron/dispatch helm job.

5. **Cache IBM MQ image** — **in progress** (`.github/actions/mq-docker-image`, `cluster-up-with-mq-image.sh`). Remaining for cache agent: digest-pin parity with `hack/kind-cluster`, kind node image load cache, document savings.

**Estimated impact:** PR job **~59 min → ~22–35 min**; `main` with Helm **~80 min → ~45–55 min** (one platform).

---

## Example Ginkgo structure (pseudo)

```go
var _ = BeforeSuite(func() {
    buildAndLoadImagesOnce()
    deployOperatorOnce() // install:crds + deploy:operator, waitForControllerAndWebhookReady once
    if mqE2EEnabled() {
        applyMQSCFixtureOnce()
    }
})

var _ = AfterSuite(func() {
    undeployOperatorOnce()
})

var _ = Describe("Manager smoke", Label("smoke"), func() {
    It("controller Ready", func() { /* no full redeploy */ })
    It("metrics 200", Label("slow"), func() { /* or move to envtest */ })
    It("webhook rejects bad Queue", func() { /* ... */ })
})

var _ = Describe("MQ reconcile", Label("mq"), func() {
    Describe("queues", Label("mq-queue"), func() {
        BeforeEach(func() { ensureNS("kurator-e2e-queues") })
        It("local queue CRUD", func() {
            prefix := uniquePrefix() // e.g. GinkgoParallelProcess()
            // queueName: E2E.<prefix>.ORDERS
        })
    })

    Describe("auth", Label("mq-auth-serial"), Serial, func() {
        BeforeEach(func() { ensureNS("kurator-e2e-auth") })
        It("CHLAUTH ADDRESSMAP on DEV.APP.SVRCONN.0TLS", func() { /* ... */ })
    })
})

// run-e2e.sh (conceptual):
// go test -tags=e2e ./test/e2e/... -ginkgo.v -ginkgo.nodes=3 \
//   -ginkgo.label-filter='(smoke || mq) && !slow'   # PR CI (manager smoke + MQ paths)
```

---

## Risks

| Risk | Mitigation |
|------|------------|
| **MQ global state** (objects persist after failed delete) | Unique names per spec; `AfterEach` best-effort delete; optional `BeforeEach` DISPLAY cleanup for prefix |
| **CHLAUTH on shared channel** | Serial `mq-auth-serial` label or dedicated channel per parallel spec |
| **Webhook / cert races** under parallel apply | Readiness `sync.Once`; `applyWithWebhookRetry` (already present); reduce concurrent applies to same CRD kind |
| **kind / Terraform flake** | Keep platform in one job step; retry mqweb wait only |
| **`-race` + parallel Ginkgo** | Run race on CI with nodes=2 first; scale up |
| **Machine lock** | CI runners are independent; local docs unchanged |
| **False confidence** if too much moves to integration | Keep ≥1 e2e path per CR kind with in-cluster `QueueManagerConnection` |

---

## Capacity: how many more tests fit?

Assume **90 min** job budget and **stable green** baseline.

| Scenario | Approx. job time | Free headroom (90 min) | Extra specs (@ ~3 min each) |
|----------|------------------|-------------------------|-----------------------------|
| **Today** (typical) | ~59 min | ~31 min | **~10** (tight; little margin for flakes) |
| **Phase A (−35% suite)** | ~48 min | ~42 min | **~14** |
| **Phase B (−50% suite)** | ~38 min | ~52 min | **~17** |
| **Phase A+B (−60% overall)** | ~32 min | ~58 min | **~19** |
| **Phase C PR path (−40% job)** | ~35 min | ~55 min | **~18** on PR; full suite on cron |

**Rule of thumb:** each **50%** cut in **suite-only** duration roughly doubles the number of **similar-complexity** MQ specs you can add before hitting the same wall-clock; platform cost caps PR gains until Phase C caches or splits Helm.

---

## References

- [`test/e2e/e2e_suite_test.go`](../../test/e2e/e2e_suite_test.go) — BeforeSuite images  
- [`test/e2e/e2e_test.go`](../../test/e2e/e2e_test.go) — Manager serial suite  
- [`test/e2e/mq_e2e_test.go`](../../test/e2e/mq_e2e_test.go) — MQ serial + Ordered contexts  
- [`test/e2e/deploy_helpers.go`](../../test/e2e/deploy_helpers.go) — deploy / webhook waits  
- [`hack/ci/run-e2e.sh`](../../hack/ci/run-e2e.sh) — no `-nodes` today  
- [`docs/CICD.md`](../CICD.md), [`docs/DEVELOPMENT.md`](../DEVELOPMENT.md)  
- [`hack/.agent-coordination.json`](../../hack/.agent-coordination.json) — `push_allowed: false` at authoring time; doc-only change does not require push.

---

## Out of scope (remaining)

- Finishing IBM MQ / kind image cache on CI runners (Phase C item 5).  
- Fixing intermittent red e2e (metrics readiness, Helm deploy).
