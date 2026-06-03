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

Run from the repository root on an up-to-date `main`:

```sh
task verify
task lint
task test:run
```

Optional but recommended before a significant release:

```sh
task ci:integration    # Docker MQ + mqweb
task ci:e2e          # kind + IBM MQ (slow)
govulncheck ./...    # or task vuln:check
```

Ensure:

- [ ] All intended PRs are merged; `main` is green in GitHub Actions.
- [ ] **CI (`ci.yaml`), integration, and e2e workflows passed on the exact commit
  SHA** you will tag — not an earlier green run on `main`.
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
VERSION=0.5.1   # replace with the tag you just published
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
