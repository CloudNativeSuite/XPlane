# Example GitOps configuration

This folder illustrates how intent is split between platform policies (GTM + autoscaling) and the infrastructure node-pool declarations that CI will eventually reconcile. Everything here is declarative—the control plane derives runtime actions such as DNS updates and Terraform commits.

- `gtm/`: Desired global traffic policy per service, including health checks and fallback weights for each region.
- `autoscale/`: Regional capacity envelopes and CPU-driven scaling rules the autoscaler reads before updating infrastructure state.
- `node-pool/`: Terraform variable files declaring the target node counts and shapes per region; autoscaler writes back here to drive CI.

The examples use `svc-plus` to demonstrate how a single service can stitch together traffic policy, scaling intent, and per-region node pools while staying cloud-neutral.
