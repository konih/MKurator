# MKurator documentation

| | Start here |
|---|------------|
| 🎯 | Install and manage MQ objects on your queue manager → [INSTALL_AND_USE.md](INSTALL_AND_USE.md) |
| 🎯 | FAQ and glossary → [FAQ.md](FAQ.md) · [GLOSSARY.md](GLOSSARY.md) |
| 📖 | Published documentation site → [conduit-ops.github.io/MKurator](https://conduit-ops.github.io/MKurator/) |
| 🛠️ | Install dev tools → [LOCAL_SETUP.md](LOCAL_SETUP.md) |
| 🛠️ | Hack on the operator locally → [DEVELOPMENT.md](DEVELOPMENT.md) |
| 🛠️ | What to regenerate / test after a code change → [DEVELOPER_GUIDE.md](DEVELOPER_GUIDE.md) |
| 🏗️ | Understand design and reconciliation → [ARCHITECTURE.md](ARCHITECTURE.md) |
| 🏗️ | Go packages, layers, and tests → [GO_MODULE.md](GO_MODULE.md) |
| 🏗️ | Manager, reconcilers, webhooks → [OPERATOR_RUNTIME.md](OPERATOR_RUNTIME.md) |
| 🤖 | AI agent conventions and workflow → [../AGENTS.md](../AGENTS.md) |
| ✉️ | Commit format (Conventional Commits + gitmoji) → [CONTRIBUTING.md](CONTRIBUTING.md) |
| 🤝 | Contributing, DCO, standards map → [../CONTRIBUTING.md](../CONTRIBUTING.md) |
| 📜 | Code of Conduct → [../CODE_OF_CONDUCT.md](../CODE_OF_CONDUCT.md) |
| 🏛️ | Governance → [../GOVERNANCE.md](../GOVERNANCE.md) |

## All docs

| | Document | Covers |
|---|----------|--------|
| 🎯 | [INSTALL_AND_USE.md](INSTALL_AND_USE.md) | Install, connect, CRs, kubectl diagnostics, upgrade, uninstall |
| 🎯 | [QUICKSTART.md](QUICKSTART.md) | Fast path: install, connect, first queue |
| 🎯 | [FAQ.md](FAQ.md) | Common questions (QMC, drift, suspend, webhooks) |
| 🎯 | [GLOSSARY.md](GLOSSARY.md) | MQ and operator terminology |
| 🎯 | [UPGRADE.md](UPGRADE.md) | Upgrade operator, CRDs, webhooks, cert-manager |
| 🎯 | [OBSERVABILITY.md](OBSERVABILITY.md) | Prometheus metrics, ServiceMonitor, RBAC |
| 🔧 | [../config/samples/README.md](../config/samples/README.md) | Annotated sample Secret, Connection, Queue, Topic, Channel, auth YAML |
| 🔧 | [../charts/mkurator/README.md](../charts/mkurator/README.md) | Helm chart to install the operator |
| 🛠️ | [LOCAL_SETUP.md](LOCAL_SETUP.md) | Install Go, Task, Docker, kind, Terraform, and other dev tools |
| 🛠️ | [DEVELOPMENT.md](DEVELOPMENT.md) | Prerequisites, inner loop, local platform, deploy, test tiers |
| 🛠️ | [DEVELOPER_GUIDE.md](DEVELOPER_GUIDE.md) | CRD codegen, reconciler/MQAdmin test matrix, mocks |
| 🛠️ | [GO_MODULE.md](GO_MODULE.md) | Module layout, import layers, `go-arch-lint` |
| 🛠️ | [OPERATOR_RUNTIME.md](OPERATOR_RUNTIME.md) | Manager, health/readiness, metrics, graceful shutdown |
| 🛠️ | [development/](development/) | Engineering standards (guidelines, coding, testing) |
| 🔒 | [ASSURANCE-CASE.md](ASSURANCE-CASE.md) | Security claims and trust boundaries |
| 🔒 | [SECURITY-REVIEW.md](SECURITY-REVIEW.md) | Dated security self-review |
| 🛠️ | [CONTRIBUTING.md](CONTRIBUTING.md) | Commit format (Conventional Commits + gitmoji) |
| 🛠️ | [RELEASE.md](RELEASE.md) | Maintainer guide: tag, changelog, publish a version |
| 🛠️ | [../hack/kind-cluster/README.md](../hack/kind-cluster/README.md) | kind + Terraform + IBM MQ platform only |
| 🛠️ | [IBM_MQ_101.md](IBM_MQ_101.md) | MQ console, `runmqsc`, verify MKurator on kind |
| 🏗️ | [ARCHITECTURE.md](ARCHITECTURE.md) | Components, CRDs, reconcile overview, security, local topology |
| 🏗️ | [GO_MODULE.md](GO_MODULE.md) | Module path, package layers, codegen, testing pyramid |
| 🏗️ | [OPERATOR_RUNTIME.md](OPERATOR_RUNTIME.md) | Manager startup, reconcile loops, cache, webhooks, errors |
| 🏗️ | [ATTRIBUTE_RECONCILIATION.md](ATTRIBUTE_RECONCILIATION.md) | DEFINE vs DISPLAY drift matrix per MQ object type |
| 🏗️ | [LOGGING.md](LOGGING.md) | Structured logging configuration and guidelines |
| 🏗️ | [adr/](adr/) | Architecture Decision Records |
| 📋 | [ROADMAP.md](ROADMAP.md) | Phased delivery plan |
| 📋 | [PHASE5_AUTH_SKETCH.md](PHASE5_AUTH_SKETCH.md) | Phase 5 CHLAUTH / AUTHREC (shipped + roadmap) |
| 📋 | [API_STABILITY.md](API_STABILITY.md) | `v1alpha1` guarantees, Phase 8 maturation, `v1beta1` graduation |
| 📋 | [NON_FUNCTIONAL_REQUIREMENTS.md](NON_FUNCTIONAL_REQUIREMENTS.md) | Security, reliability, observability, performance |
| 📋 | [CICD.md](CICD.md) | CI/CD pipeline and `verify` discipline |
| 🔒 | [../SECURITY.md](../SECURITY.md) | Security posture and vulnerability reporting |
| 📚 | [IBM_MQ_OBJECTS.md](IBM_MQ_OBJECTS.md) | MQSC research inventory (**not** the product API) |
| 📚 | [IBM_MQ_REST_API.md](IBM_MQ_REST_API.md) | How the `mqweb` REST API is consumed |
| 📚 | [schemas/README.md](schemas/README.md) | mqweb Swagger / MQSC JSON schemas |
| 📚 | [REFERENCES.md.example](REFERENCES.md.example) | Optional map of vendored `references/` clones (copy to `REFERENCES.md`) |

**Contract vs research:** shipped behaviour is defined by v1alpha1 CRDs and
[ATTRIBUTE_RECONCILIATION.md](ATTRIBUTE_RECONCILIATION.md), not
[IBM_MQ_OBJECTS.md](IBM_MQ_OBJECTS.md).

## Emoji key (navigation only)

| Emoji | Meaning |
|-------|---------|
| 🎯 | End-user / operator |
| 🛠️ | Developer workflow |
| 🏗️ | Architecture and design |
| 📋 | Roadmap, NFRs, CI |
| 📚 | IBM MQ reference / research |
| 🔧 | Samples, Helm, config |
| 🔒 | Security |
