# Sample resources (Helm)

Same manifests as [`config/samples/`](../../config/samples/README.md), used by
`task deploy:samples` after `task deploy:helm`.

See [docs/INSTALL_AND_USE.md](../../docs/INSTALL_AND_USE.md) for the full install
and usage guide.

| File | Kind |
|------|------|
| `mq-credentials-secret.yaml` | `Secret` |
| `queuemanagerconnection.yaml` | `QueueManagerConnection` |
| `queue.yaml` | `Queue` |

Apply order: Secret → `QueueManagerConnection` → wait for `Ready` → `Queue`.
