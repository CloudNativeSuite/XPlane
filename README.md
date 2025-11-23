# XPlane

XPlane is a lightweight, cloud-neutral control plane designed for global-scale applications without the complexity of Kubernetes or vendor-locked platforms.

## Narrative overview
For builders who need a durable platform that runs anywhere, XPlane keeps the control loop small and transparent. It relies on a single control node and containerd-only workloads, pushing all infrastructure mutations through Git + CI so your platform state is always auditable and reproducible. The goal: multi-region resilience without heavyweight orchestration.

## Core capabilities
- **Global Traffic Management (GTM):** dynamic DNS, health-aware GSLB, and intelligent region failover to keep traffic flowing to healthy endpoints.
- **Autoscaling Reconciler:** computes the desired infrastructure state, updates Git, and lets CI drive scaling actions predictably.
- **GitOps Synchronization:** treats declarative platform configs as the single source of truth so environments stay consistent across clouds and regions.

## Why XPlane
- **Cloud-neutral by design:** works across multi-cloud, hybrid, bare-metal, and edge footprints.
- **Lean control surface:** one control node that coordinates everything without a Kubernetes dependency.
- **Resilient defaults:** region-aware failover paired with Git-driven change management.
- **Operational clarity:** observability through Git history and focused reconciliation loops.

## Running with the essentials
XPlane lets you run a full multi-region platform using only:

- One control node
- Containerd-only workloads
- Git + CI for all infrastructure mutations

Start here, evolve incrementally, and keep your platform portable.
