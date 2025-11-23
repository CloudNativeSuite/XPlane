# Example GitOps configuration

This directory mirrors the `platform-config` layout described in the architecture notes so Git remains the single source of intent for GTM, autoscaling, and regional topology. Each subfolder is intentionally empty aside from a placeholder so teams can drop declarative policies that the control plane will ingest.

- `gtm/` holds desired DNS, weighting, and health-aware traffic policies.
- `autoscale/` captures min/max capacity, scale rules, and any thresholds the autoscaler will use before committing changes back to infrastructure code.
- `regions/` enumerates the regions and node pools that make up the global footprint.

See the architecture document for the broader repository layout and principles separating platform intent from infrastructure execution and node configuration.
