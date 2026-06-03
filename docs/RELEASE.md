# Releasing a new version

Step-by-step guide for maintainers publishing a **Kurator** release. Consumers
install published artifacts via [INSTALL_AND_USE.md](INSTALL_AND_USE.md).

Related: [CONTRIBUTING.md](CONTRIBUTING.md#changelog-and-releases) (commits),
[CICD.md](CICD.md) (CI behaviour), [ADR-0008](adr/0008-changelog-git-cliff.md),
[ADR-0016](adr/0016-release-supply-chain.md).

## On this page

| | Section |
|---|---------|
| 📋 | [Overview](#overview) |
| ✅ | [Pre-release checklist](#pre-release-checklist) |
| 🏷️ | [Version and changelog](#version-and-changelog) |
| 🚀 | [Tag and publish](#tag-and-publish) |
| 📦 | [What CI publishes](#what-ci-publishes) |
| 🔧 | [Rebuild or fix a failed release](#rebuild-or-fix-a-failed-release) |
| ✔️ | [Verify after release](#verify-after-release) |

## Overview

Releases are **tag-driven**: push an annotated-style lightweight tag `vX.Y.Z` on
`main` and [`.github/workflows/release.yaml`](../.github/workflows/release.yaml)
builds, scans, signs, and publishes artifacts. Version numbers are **not** bumped
by CI — you commit `Chart.yaml` and `CHANGELOG.md` on `main` first.

Semver on `0.y.z` while the API is `v1alpha1`: use **minor** (`0.3.0`) for
user-visible features or **breaking** operator/API behaviour; **patch** (`0.2.3`)
for fixes only. Breaking commits use `!` in the subject (see
[CONTRIBUTING.md](CONTRIBUTING.md#breaking-changes)).

## Pre-release checklist

Record the commit you will tag (after the prepare commit, if any):

```sh
git checkout main && git pull
RELEASE_SHA="$(git rev-parse HEAD)"
echo "Tagging: ${RELEASE_SHA}"
```

Run from the repository root on that commit:

```sh
task verify
task lint
task test:run
```

Before publishing a release tag, also run (or confirm green on GitHub for
`${RELEASE_SHA}`):

```sh
task ci:integration    # Docker MQ + mqweb — workflow integration.yaml
task ci:e2e            # kind + IBM MQ — workflow e2e.yaml (slow)
task vuln:check        # same as CI test job
```

**Before tag:** confirm **E2E** and **Integration** (and full **CI**) are green on
`${RELEASE_SHA}` — not only “latest green on `main`” from an older push.

### Automated release gate workflow

Use [`.github/workflows/release-gate.yaml`](../.github/workflows/release-gate.yaml)
(**Actions → Release gate → Run workflow**) on the commit you will tag:

| Input | Meaning |
|-------|---------|
| `sha` | Full or short commit (empty = latest `main` HEAD) |
| `poll_timeout_minutes` | How long to wait for external check-runs (default 120) |

The workflow re-runs `task verify`, `task test:run`, and Docker MQ integration on
that SHA, then polls GitHub check-runs until **CI**, **Integration**, and **E2E
(kustomize)** jobs succeeded on the same SHA. E2E is not run inside this workflow
(~90 min); you must already have (or wait for) a green **E2E** workflow run whose
`headSha` matches `${RELEASE_SHA}`. Record the E2E run ID or URL when tagging.

If polling fails (timeout, skipped workflows, API limits), the **manual e2e
checklist** job fails with `gh run list` / `gh run view` commands — do not tag
until E2E is green on the exact SHA.

### CI gate on the exact SHA

Do **not** tag until all three workflow runs succeeded on **`${RELEASE_SHA}`**
(the commit at `HEAD` when you create the tag), not merely “latest green on
`main`” from an older push.

| Workflow | File | What it proves |
|----------|------|----------------|
| **CI** | [`.github/workflows/ci.yaml`](../.github/workflows/ci.yaml) | `verify`, `lint`, `test` (+ Codecov upload), `build`, `docker-build`, `helm-lint` |
| **Integration** | [`.github/workflows/integration.yaml`](../.github/workflows/integration.yaml) | Live mqweb: queues, topics, channels, CHLAUTH, AUTHREC |
| **E2E** | [`.github/workflows/e2e.yaml`](../.github/workflows/e2e.yaml) | Operator on kind + IBM MQ (Kustomize deploy) |

On GitHub: **Actions** → select the workflow → open the latest run on `main` →
confirm the commit SHA matches `${RELEASE_SHA}` (copy from `git rev-parse` or
the run header). With the GitHub CLI:

```sh
gh run list --workflow ci.yaml --branch main --limit 5
gh run list --workflow integration.yaml --branch main --limit 5
gh run list --workflow e2e.yaml --branch main --limit 5
# Inspect a run: gh run view <run-id> --json headSha,conclusion,status
```

Ensure:

- [ ] All intended PRs are merged; `main` is at `${RELEASE_SHA}`.
- [ ] **CI**, **Integration**, and **E2E** workflows show **success** for
  `${RELEASE_SHA}` (re-run workflows on that commit if needed before tagging).
- [ ] Commits since the last tag follow [Conventional Commits + gitmoji](CONTRIBUTING.md#commit-message-format).
- [ ] No accidental secrets in the tree (`task secrets:scan` if unsure).
- [ ] If CRDs/webhooks/RBAC changed: `task manifests` and `task verify` already clean.

## Version and changelog

### 1. Preview unreleased notes

```sh
task changelog
```

Review grouping (Features, Bug Fixes, Breaking Changes). Skipped types: `docs`,
`test`, `chore`, `ci`, `build`, `style` — see [`cliff.toml`](../cliff.toml).

### 2. Choose the version

| Change | Example bump |
|--------|----------------|
| Breaking (`feat!`, `refactor!`, CRD contract) | `0.2.2` → `0.3.0` |
| New feature, non-breaking | `0.2.2` → `0.3.0` or `0.2.3` |
| Bug fixes only | `0.2.2` → `0.2.3` |

### 3. Bump the Helm chart

Edit [`charts/kurator/Chart.yaml`](../charts/kurator/Chart.yaml):

```yaml
version: 0.3.0
appVersion: "0.3.0"
```

Keep `version` and `appVersion` aligned with the git tag (`v0.3.0` → `0.3.0`).

### 4. Regenerate CHANGELOG.md

```sh
task changelog:write
```

Commit chart bump and changelog together:

```sh
git add charts/kurator/Chart.yaml CHANGELOG.md
git commit -m "chore(release): :bookmark: prepare v0.3.0"
```

Use a conventional subject; the release tag itself does not need to be in this commit.

## Tag and publish

Create and push the tag (triggers CI):

```sh
git tag v0.3.0
git push origin main
git push origin v0.3.0
```

CI runs only when the tag matches `v*.*.*` (see workflow `on.push.tags`).

**Do not** move or force-push release tags once consumers may have pulled them.
Fix forward with a new patch tag instead.

## What CI publishes

The [release workflow](https://github.com/konih/kurator/actions/workflows/release.yaml)
(on tag push):

| Output | Location |
|--------|----------|
| Container image | `ghcr.io/konih/kurator:0.3.0` (and `:v0.3.0`), multi-arch |
| OCI SBOM + SLSA provenance | GHCR attestations on the image |
| GitHub Release | Notes from git-cliff + install section; attached files below |
| `install-crds.yaml` | Kustomize CRD bundle |
| `install.yaml` | Full operator install (image pinned to tag) |
| `kurator-0.3.0.tgz` | Helm chart tarball |
| `sbom.spdx.json` | SPDX SBOM |
| `checksums.txt` | SHA256 of release files |
| Helm chart (OCI) | `oci://ghcr.io/konih/kurator` |

Release notes are assembled by
[`hack/assemble-release-notes.sh`](../hack/assemble-release-notes.sh) (git-cliff
section + [`.github/release-notes-install.md`](../.github/release-notes-install.md)).

Local dry-run of install manifests (without pushing):

```sh
bash hack/release-assets.sh 0.3.0 ghcr.io/konih/kurator
ls -la dist/
```

## Rebuild or fix a failed release

### Trivy or build failed on tag

1. Fix the issue on `main` (dependency bump, `.trivyignore` with rationale, Dockerfile).
2. Bump **patch** version if the tag was never successfully consumed, or publish
   a new tag (e.g. `v0.3.1`).
3. Do **not** delete the failed tag on a public repo unless you are certain no one
   pulled it.

### Rebuild assets for an existing tag (testing)

Use **workflow_dispatch** on the Release workflow with the existing tag name
(e.g. `v0.3.0`). This rebuilds and re-uploads GitHub Release assets without a new commit.

### Changelog wrong on GitHub Release

Edit `cliff.toml` or commit messages on `main` only affect **future** releases.
For a published release, edit release notes manually on GitHub or re-run
workflow_dispatch after fixing generation on the tagged commit.

## Verify after release

1. Open [GitHub Releases](https://github.com/konih/kurator/releases) — notes, attachments, tag.
2. Pull the image: `docker pull ghcr.io/konih/kurator:0.3.0`
3. Optional cosign verify (substitute digest from GHCR):

```sh
cosign verify \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp '^https://github.com/konih/kurator/.+' \
  ghcr.io/konih/kurator@sha256:<digest>
```

4. Smoke install from release YAML (see [INSTALL_AND_USE.md](INSTALL_AND_USE.md)):

```sh
VERSION=0.5.2   # replace with the tag you just published
curl -sLO "https://github.com/konih/kurator/releases/download/v${VERSION}/install-crds.yaml"
curl -sLO "https://github.com/konih/kurator/releases/download/v${VERSION}/install.yaml"
kubectl apply -f install-crds.yaml
kubectl apply -f install.yaml
```

5. Confirm `CHANGELOG.md` on `main` no longer lists released items under
   **Unreleased** (regenerate was done pre-tag; post-tag commits may add new unreleased).

## Quick reference

```sh
task changelog
# edit charts/kurator/Chart.yaml → version + appVersion
task changelog:write
git add charts/kurator/Chart.yaml CHANGELOG.md
git commit -m "chore(release): :bookmark: prepare vX.Y.Z"
git tag vX.Y.Z
git push origin main && git push origin vX.Y.Z
```
