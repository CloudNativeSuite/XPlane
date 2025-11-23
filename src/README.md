# Source layout

The source tree mirrors the three core control-plane loops described in the architecture notes and keeps binaries thin at the edge:

- `cmd/` will hold entrypoints for control-plane services and supporting tooling.
- `pkg/gitops/` encapsulates syncing desired state from Git and notifying downstream reconcilers.
- `pkg/gtm/` and `pkg/controller/` provide the health-aware DNS reconciliation and traffic management logic.
- `pkg/autoscaler/` tracks metrics and computes desired node counts before persisting updates through Git-driven workflows.
- `pkg/provider/` isolates provider-specific APIs (e.g., DNS vendors or infrastructure backends) so the control plane stays cloud-neutral.
- `internal/` is reserved for shared helpers and platform-scoped utilities.

Each package starts empty with placeholders so future commits can focus on implementing the reconciliation flows without reworking the project shape.
