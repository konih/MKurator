# Architecture Decision Records

This directory holds **Architecture Decision Records (ADRs)**: short documents
that capture a significant decision, its context, and its consequences. They are
the durable record of *why* the project looks the way it does.

## When to write one

Write an ADR when a decision is non-obvious, hard to reverse, or likely to be
questioned later — e.g. choice of framework, an external protocol, an API
shape, or a tooling trade-off. Don't write one for routine, easily reversible
choices.

## How to add one

1. Copy [`0000-template.md`](0000-template.md) to the next number, e.g.
   `0005-short-title.md`.
2. Fill in Context, Decision, Consequences, and Alternatives.
3. Set the status (`Proposed` → `Accepted` → optionally `Superseded by ADR-NNNN`).
4. Add it to the index below and link it from the relevant doc/code.

ADRs are immutable once Accepted: to change a decision, write a new ADR that
supersedes the old one rather than editing history.

## Index

| ADR | Title | Status |
|-----|-------|--------|
| [0001](0001-use-kubebuilder-controller-runtime.md) | Use Kubebuilder + controller-runtime | Accepted |
| [0002](0002-manage-mq-via-mqweb-rest.md) | Manage MQ via the mqweb REST API behind an MQAdmin port | Accepted |
| [0003](0003-connection-model.md) | Decouple connection details with QueueManagerConnection | Accepted |
| [0004](0004-task-as-task-runner.md) | Use Task as the task runner | Accepted |
| [0005](0005-keep-tooling-lean.md) | Keep tooling lean; borrow discipline, not org overhead | Accepted |
| [0006](0006-project-name-kurator.md) | Project name and module identity (Kurator) | Accepted |
| [0007](0007-structured-logging-logr-slog.md) | Structured logging with logr and slog | Accepted |
| [0008](0008-changelog-git-cliff.md) | Generate changelogs with git-cliff | Accepted |
| [0009](0009-validating-admission-webhooks.md) | Validating admission webhooks (no MQ at admission) | Accepted |
| [0010](0010-drift-based-mq-reconciliation.md) | Drift-based MQ reconciliation (DEFINE + DISPLAY) | Accepted |
| [0011](0011-layered-testing-strategy.md) | Layered testing strategy (mock → envtest → MQ → kind) | Accepted |
| [0012](0012-operator-scope-existing-queue-manager.md) | Operator scope — existing Queue Manager only | Accepted |
| [0013](0013-finalizers-and-deletion.md) | Finalizers and deletion (MQ object before CR removal) | Accepted |
| [0014](0014-mq-error-taxonomy-and-requeue.md) | MQ error taxonomy and requeue strategy | Accepted |
| [0015](0015-kubernetes-events-on-transitions.md) | Kubernetes Events on status transitions only | Accepted |
| [0016](0016-release-supply-chain.md) | Release supply chain (image, SBOM, signing, scan) | Accepted |
