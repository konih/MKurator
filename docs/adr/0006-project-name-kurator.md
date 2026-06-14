# ADR-0006: Project name and module identity (Kurator)

- **Status**: Superseded by [ADR-0018](0018-project-rename-mkurator.md)
- **Date**: 2026-06-02

## Context

The operator needed a memorable project name before scaffolding (module path, API
group, image names, kind cluster defaults). The working title was "IBM Message
Queue Operator" with placeholder identifiers (`ibm-mq-operator`,
`messaging.heimel.dev`).

## Decision

- **Project name**: **Kurator** (from "curator" — declarative curation of MQ
  resources on a Queue Manager).
- **Go module**: `github.com/conduit-ops/mkurator`
- **GitHub / GHCR**: `github.com/conduit-ops/MKurator`, `ghcr.io/conduit-ops/mkurator`
- **API group / domain**: `messaging.kurator.dev`, version `v1alpha1`
- **Local kind cluster** default name: `kurator`
- **Container image** (local): `kurator-controller-manager:latest`

IBM MQ remains the *target system*; the operator name does not imply an IBM
product affiliation.

## Consequences

- All docs, samples, RBAC markers, and codegen used `messaging.kurator.dev`.
- Existing local clusters created as `ibm-mq-operator` keep working if
  `CLUSTER_NAME=ibm-mq-operator` is set; new bring-ups used `kurator`.
- Repository directory name may stay `IBM-Message-Queue-Operator` until a rename
  is convenient; the Go module path is authoritative.

## Alternatives considered

- **Qurator** — same meaning, alternate spelling; rejected in favour of the
  simpler **Kurator** spelling the maintainer chose.
- **ibm-mq-operator** — descriptive but long and tied to IBM branding in the
  module path.
