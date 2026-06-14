# Commit conventions

> **Process overview** (standards map, DCO, merge policy, expectations) →
> [../CONTRIBUTING.md](../CONTRIBUTING.md)

Developer guidelines for **MKurator**: commit message format, gitmoji, changelog, and
release notes. For local setup see [DEVELOPMENT.md](DEVELOPMENT.md); for Go style,
testing, and agent workflow see [AGENTS.md](../AGENTS.md).

Doc index: [README.md](README.md)

## On this page

| | Section |
|---|---------|
| ✉️ | [Commit message format](#commit-message-format) |
| 🏷️ | [Types and scopes](#types-and-scopes) |
| 😀 | [Gitmoji](#gitmoji) |
| 💥 | [Breaking changes](#breaking-changes) |
| 📝 | [Examples](#examples) |
| 📰 | [Changelog and releases](#changelog-and-releases) |
| ✅ | [Before you open a PR or push](#before-you-open-a-pr-or-push) |

## Commit message format

Every commit uses **[Conventional Commits](https://www.conventionalcommits.org/)**
plus a **[gitmoji](https://gitmoji.dev/)** code in the subject.

```text
<type>(<optional scope>): :<gitmoji>: <short summary>

<optional body>

<optional footer>
```

| Part | Rules |
|------|--------|
| **type** | Required. Lowercase. See [types](#types-and-scopes). |
| **scope** | Optional but encouraged when the change is localized (e.g. `mqrest`, `webhook`). |
| **gitmoji** | Required. ASCII shortcode immediately after the first colon, before the summary (`:sparkles:`, not the Unicode emoji). |
| **summary** | Imperative mood, lowercase start, no trailing period, ~50 characters or less. |
| **body** | Optional. Wrap at ~72 columns. Explain *what* and *why*, not file lists. |
| **footer** | Optional. Breaking changes, issue refs (`Closes #123`). |

**Subject line regex (informal):**

```text
^(feat|fix|docs|style|refactor|test|chore|ci|build)(\([^)]+\))?!?: :[a-z0-9_]+: .+$
```

Release notes are generated from these subjects by [git-cliff](https://git-cliff.org/)
([ADR-0008](adr/0008-changelog-git-cliff.md)); malformed commits are skipped or
grouped incorrectly.

## Types and scopes

### Types

| Type | When to use | In user-facing changelog? |
|------|-------------|---------------------------|
| `feat` | New behaviour users or operators care about | Yes (Features) |
| `fix` | Bug fix | Yes (Bug Fixes) |
| `refactor` | Code change without fixing a bug or adding a feature | Yes (Refactoring) |
| `perf` | Performance improvement | Yes |
| `docs` | Documentation only | No (skipped by default) |
| `test` | Tests only | No |
| `chore` | Tooling, deps, repo hygiene | No |
| `ci` | CI/CD workflows | No |
| `build` | Build system, Dockerfile, Taskfile | No |
| `style` | Formatting, whitespace (no logic change) | No |

### Scopes (suggested)

Use a scope when it helps readers and changelog grouping. Common scopes in this
repo:

| Scope | Typical area |
|-------|----------------|
| `controller` | `internal/controller` reconcilers |
| `mqrest` | `internal/adapter/mqrest` REST client |
| `webhook` | `internal/webhook`, `internal/validation` |
| `queue`, `topic`, `channel`, `messaging` | CR-specific reconcile or API |
| `chart` | `charts/mkurator` Helm chart |
| `ci` | `.github/workflows` |
| `docs` | `docs/`, README |
| `test` | `test/`, `*_test.go` |
| `cluster` | `hack/kind-cluster` local platform |

Omit scope only when the change truly spans the tree (e.g. `chore: :wrench: bump Go to 1.26`).

## Gitmoji

**Required:** every subject includes exactly one gitmoji **shortcode** between the
first colon and the summary:

```text
feat(queue): :sparkles: reconcile QLOCAL via mqweb
           ^  ^^^^^^^^^^
           |  gitmoji (required)
           type + optional scope
```

Use the [gitmoji](https://gitmoji.dev/) meaning that best matches the change—not
decorative emoji. Prefer the table below; the full catalogue is on gitmoji.dev.

| Gitmoji | Code | Use for |
|---------|------|---------|
| ✨ | `:sparkles:` | New feature |
| 🐛 | `:bug:` | Bug fix |
| 📝 | `:memo:` | Documentation |
| ✅ | `:white_check_mark:` | Add, update, or fix tests |
| ♻️ | `:recycle:` | Refactor |
| 🔧 | `:wrench:` | Configuration files (Taskfile, YAML config, Helm values) |
| 👷 | `:construction_worker:` | CI build system |
| 🧱 | `:bricks:` | Infrastructure / platform (kind, Terraform, Docker MQ) |
| 🙈 | `:see_no_evil:` | `.gitignore` or ignore rules |
| ⬆️ | `:arrow_up:` | Upgrade dependency |
| ⬇️ | `:arrow_down:` | Downgrade dependency |
| 🔒 | `:lock:` | Security |
| 🚑️ | `:ambulance:` | Critical hotfix |
| 🎨 | `:art:` | Improve structure/format of code (style) |
| ⚡ | `:zap:` | Performance |
| 🔥 | `:fire:` | Remove code or files |
| ✏️ | `:pencil2:` | Fix typos |

**Do not:**

- Put the Unicode emoji in the subject instead of the shortcode (`feat: ✨ add` — wrong).
- Omit the gitmoji (`feat(queue): add reconcile` — wrong).
- Use multiple gitmojis in one subject.

## Breaking changes

API or behaviour breaks for consumers (CRD schema, reconcile semantics, install
manifests) must be visible in the commit and changelog.

1. Add `!` after the type or scope: `feat(api)!: :sparkles: rename spec field`.
2. Describe migration in the **body** or a `BREAKING CHANGE:` footer (Conventional
   Commits style).

```text
refactor!: :recycle: rename module to github.com/conduit-ops/MKurator

BREAKING CHANGE: import paths and container image registry moved to conduit-ops/MKurator.
```

Breaking commits appear under **Breaking Changes** in [CHANGELOG.md](../CHANGELOG.md).

## Examples

**Good:**

```text
feat(queue): :sparkles: reconcile Queue into MQSC DEFINE QLOCAL
fix(mqrest): :bug: retry on 5xx from mqweb admin endpoint
docs: :memo: document QueueManagerConnection secret reference
test(controller): :white_check_mark: add envtest for deletion finalizer
ci: :construction_worker: pin git-cliff-action to v4.8.0
chore(deps): :arrow_up: bump controller-runtime to v0.23.3
```

**Bad:**

```text
fixed queue bug                    # no type, no gitmoji
feat: add queue reconcile          # missing gitmoji
feat(queue): add queue reconcile   # missing gitmoji shortcode
feat(queue): ✨ add reconcile       # Unicode emoji instead of :sparkles:
WIP                                # not conventional
feat(queue): :sparkles: Fixed the thing.  # past tense, trailing period
```

## Changelog and releases

[`CHANGELOG.md`](../CHANGELOG.md) is generated from git history, not hand-written
per bullet.

| Task | Purpose |
|------|---------|
| `task changelog` | Preview the **Unreleased** section |
| `task changelog:write` | Regenerate full `CHANGELOG.md` |
| `task changelog:release` | Print notes for the latest tag |

**Maintainer release flow** — full runbook: [RELEASE.md](RELEASE.md). Summary:

1. Merge work on `main` with conventional commits.
2. `task changelog` — sanity-check grouping.
3. Bump `charts/mkurator/Chart.yaml` `version` and `appVersion`.
4. `task changelog:write` — commit `CHANGELOG.md`.
5. `git tag vX.Y.Z && git push origin vX.Y.Z` — CI publishes image and GitHub Release.

Only `feat`, `fix`, `perf`, `refactor`, and breaking commits appear in the
user-facing changelog; `docs` / `test` / `chore` / `ci` / `build` / `style` are
skipped ([`cliff.toml`](../cliff.toml)).

## Before you open a PR or push

See [../CONTRIBUTING.md](../CONTRIBUTING.md) for the full preflight checklist and merge policy.
Minimum before push:

1. `task verify` — CRDs, RBAC, deepcopy, mocks are up to date.
2. `task lint` — golangci-lint clean.
3. `task test:run` — unit + envtest green (`-race`).
4. Commit message follows [commit message format](#commit-message-format) with DCO sign-off when able.

## Further reading

| Doc | Topic |
|-----|--------|
| [../CONTRIBUTING.md](../CONTRIBUTING.md) | Standards map, DCO, code review |
| [DEVELOPMENT.md](DEVELOPMENT.md) | Local setup, Task commands, test tiers |
| [AGENTS.md](../AGENTS.md) | Go conventions, codegen, CI parity |
| [CICD.md](CICD.md) | Pipeline and release job |
| [RELEASE.md](RELEASE.md) | Maintainer release runbook |
| [adr/0008-changelog-git-cliff.md](adr/0008-changelog-git-cliff.md) | Why git-cliff |
| [SECURITY.md](../SECURITY.md) | Reporting vulnerabilities |
