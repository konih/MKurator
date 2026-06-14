# Contributing to MKurator

Thank you for helping improve MKurator.

This project follows the [Code of Conduct](CODE_OF_CONDUCT.md) and is governed per
[GOVERNANCE.md](GOVERNANCE.md).

## Standards map

Pull requests must meet the linked standards before merge. Each document owns one concern — do not
duplicate prose across them.

| Document | Owns |
| --- | --- |
| [NON_FUNCTIONAL_REQUIREMENTS.md](docs/NON_FUNCTIONAL_REQUIREMENTS.md) | Product *what* — security, reliability, observability NFRs |
| [Engineering guidelines](docs/development/guidelines.md) | Operator *how well* — error taxonomy, TLS/credentials, robustness, definition of done |
| [Coding standards](docs/development/coding-standards.md) | Go *how* — lint, formatting, modules, race detector, CI gates |
| [Testing strategy](docs/development/testing.md) | Test pyramid (L0–L5), coverage floors, integration/e2e tiers |
| [Commit conventions](docs/CONTRIBUTING.md) | Gitmoji + Conventional Commits detail, changelog, release flow |
| [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) | Community behavior standards (Contributor Covenant v2.1) |
| [GOVERNANCE.md](GOVERNANCE.md) | Roles, decision making, continuity, security contact |
| [SECURITY.md](SECURITY.md) | Vulnerability reporting |
| [Assurance case](docs/ASSURANCE-CASE.md) | Security claims, trust boundaries, countermeasures |
| [Security review](docs/SECURITY-REVIEW.md) | Dated self-review findings and residual risks |
| [SCA remediation policy](docs/security/sca-remediation-policy.md) | Dependency CVE and license remediation thresholds |
| [Architecture decision records](docs/adr/) | Locked design decisions — update or add ADRs for non-trivial changes |
| [tooling-setup.md](docs/development/tooling-setup.md) | Maintainer setup for arch-lint, depguard, SonarCloud |
| [AGENTS.md](AGENTS.md) | AI agent conventions and workflow |

**Merge policy:** `main` requires green **`preflight`** and **`CI`** checks.

**Local preflight** (before opening a PR): `task verify` · `task lint` · `task test:run` ·
`task scrub` · `gitleaks protect --staged --no-banner`.

## Expectations

- **One logical change per commit** (or per PR). The tree should build, lint, and pass
  unit/envtest at each commit you share.
- **Small, reviewable diffs** over large drive-by refactors. Match existing patterns in the
  package you touch.
- **Tests with behaviour changes** — see [testing strategy](docs/development/testing.md) and
  [DEVELOPMENT.md](docs/DEVELOPMENT.md#test-tiers). A fix or feature is not done until the right
  tier is updated.
- **Generated artifacts stay fresh** — run `task generate && task manifests` when APIs or
  kubebuilder markers change, then `task verify` before pushing.
- **Sample CR YAML** — edit [`config/samples/`](config/samples/) first, then `task samples:sync`
  so [`charts/mkurator/samples/resources/`](charts/mkurator/samples/resources/) stays in sync.
- **No secrets in git** — credentials belong in cluster Secrets, not commits or logs.

Personal project: no JIRA keys in subjects. Use English for commit messages and user-facing docs.

## Commit messages

Full gitmoji + Conventional Commits rules: [docs/CONTRIBUTING.md](docs/CONTRIBUTING.md).

Every commit subject uses the format:

```text
<type>(<optional scope>): :<gitmoji>: <short summary>
```

Release notes are generated from these subjects by [git-cliff](https://git-cliff.org/)
([ADR-0008](docs/adr/0008-changelog-git-cliff.md)).

## Developer Certificate of Origin (DCO)

By contributing, you certify the Developer Certificate of Origin (DCO) (version 1.1):

```text
Developer Certificate of Origin
Version 1.1

Copyright (C) 2004, 2006 The Linux Foundation and its contributors.

Everyone is permitted to copy and distribute verbatim copies of this
license document, but changing it is not allowed.

Developer's Certificate of Origin 1.1

By making a contribution to this project, I certify that:

(a) The contribution was created in whole or in part by me and I
    have the right to submit it under the open source license
    indicated in the file; or

(b) The contribution is based upon previous work that, to the best
    of my knowledge, is covered under an appropriate open source
    license and I have the right under that license to submit that
    work with modifications, whether created in whole or in part
    by me, under the same open source license (unless I am
    permitted to submit under a different license), as indicated
    in the file; or

(c) The contribution was provided directly to me by some other
    person who certified (a), (b) or (c) and I have not modified
    it.

(d) I understand and agree that this project and the contribution
    are public and that a record of the contribution (including all
    personal information I submit with it, including my sign-off) is
    maintained indefinitely and may be redistributed consistent with
    this project or the open source license(s) involved.
```

Include a `Signed-off-by` line in every commit message when you are able:

```text
feat(queue): :sparkles: reconcile QLOCAL via mqweb

Signed-off-by: Your Name <your.email@example.com>
```

Git can append this automatically: `git commit -s`. The DCO is a statement of license on your
contribution; it complements the MIT license in [LICENSE](LICENSE). A DCO bot is not required —
maintainers may ask you to amend commits if sign-off is missing on substantive contributions.

## Reporting bugs

**Open a [GitHub Issue](https://github.com/conduit-ops/MKurator/issues/new)** for bugs, regressions, and
feature requests. Do **not** use issues for security vulnerabilities — email
**konrad.heimel@gmail.com** per [SECURITY.md](SECURITY.md).

Include: MKurator version or commit, Kubernetes version, IBM MQ version if relevant, minimal repro
YAML or steps, expected vs actual behavior, and relevant operator logs (redact secrets).

## Code review

All pull requests need **green CI** and **maintainer approval** before merge to `main`.

### Required checks

| Check | Task / workflow |
| --- | --- |
| Codegen drift | `task verify` (CI `verify` job) |
| Lint | `task lint` (CI `lint` job) |
| Unit + envtest | `task test:run` (CI `test` job) |
| Secrets | gitleaks (CI `gitleaks` job) |

See [CICD.md](docs/CICD.md) for the full workflow matrix.

## Changelog and releases

[`CHANGELOG.md`](CHANGELOG.md) is generated from git history. Maintainer release flow:
[docs/RELEASE.md](docs/RELEASE.md).

## License

By contributing, you agree that your contributions are licensed under the
[MIT License](LICENSE) and that you certify the [DCO](#developer-certificate-of-origin-dco) as
described above.

All participants are expected to follow the [Code of Conduct](CODE_OF_CONDUCT.md).

## Further reading

| Doc | Topic |
| --- | --- |
| [DEVELOPMENT.md](docs/DEVELOPMENT.md) | Local setup, Task commands, test tiers |
| [DEVELOPER_GUIDE.md](docs/DEVELOPER_GUIDE.md) | Change matrix — what to regenerate after edits |
| [CICD.md](docs/CICD.md) | Pipeline and release job |
| [docs/CONTRIBUTING.md](docs/CONTRIBUTING.md) | Commit format, gitmoji, examples |
