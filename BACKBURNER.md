# Backburner Ideas

Last updated: February 23, 2026

## Deferred

- Separate pricing semantics for subscription vs API usage:
  - `cburn` should model Claude subscription economics independently from Admin/API token pricing.
  - Admin Cost API integration is still valuable for API-key workflows, but should not be treated as the canonical source for subscription usage.
  - Revisit after daemon/event infrastructure work stabilizes.
