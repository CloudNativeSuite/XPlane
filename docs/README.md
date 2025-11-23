# XPlane Documentation

## Platform overview
XPlane is a lightweight, cloud-neutral control plane purpose-built for teams that need global reach without the overhead of Kubernetes or vendor-specific stacks. It keeps the control loop minimal by using a single control node that drives declarative configs through Git + CI, ensuring every infrastructure change is tracked and repeatable.

## Core capabilities
- **Global Traffic Management (GTM):** orchestrates dynamic DNS, health-aware GSLB, and region failover to keep requests routed to healthy endpoints.
- **Autoscaling Reconciler:** evaluates workload demand, computes the desired infrastructure state, and commits updates to Git for CI-driven actions.
- **GitOps Synchronization:** treats declarative platform configurations as the source of truth so environments stay aligned across clouds and regions.

## Operating model
1. Define platform and application configuration as code in Git.
2. Let the autoscaling reconciler compute desired state and push commits that CI applies to your infrastructure.
3. Rely on GTM to steer traffic globally with health and latency awareness.
4. Keep workloads containerd-native to stay portable across multi-cloud, hybrid, bare-metal, and edge footprints.

This documentation set will expand with guides, examples, and integration notes as the platform evolves.
