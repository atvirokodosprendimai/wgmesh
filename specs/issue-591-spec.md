# Specification: Issue #591

## Classification
wont-do

## Deliverables
documentation

## Problem Analysis

Issue #591 requests a full customer onboarding automation suite: automated welcome email
sequences, a progressive onboarding checklist in a "customer portal", quick-start
configuration templates, integration health checks, and a human-support escalation path.

The current product state makes this premature:

1. **No email infrastructure.** The wgmesh repository contains no SMTP client, transactional
   email provider integration (Resend, SendGrid, Postmark, etc.), or templating engine. Adding
   one is a multi-week cross-repo build.

2. **No customer portal.** Billing is fully delegated to Polar.sh. There is no wgmesh-owned
   web UI today; the only operator surface is the CLI. A "progressive onboarding checklist"
   has nowhere to live.

3. **Human-support escalation path.** The team is at the stage of finding a *first* paying
   customer (ROADMAP.md "Bet A"). Building a tiered support escalation system at this point
   optimises for a scale that does not yet exist.

4. **Roadmap position.** ROADMAP.md explicitly places customer success tooling in
   **Bet C — Hundredth paying customer (Q4–Q1)**:
   > *"Mobile, SSO, audit logs, GDPR/SOC 2 posture, plugin ecosystem, full-write dashboard."*
   > *"somewhere around customer 30 we stop selling to homelabbers and start selling to small
   > companies."*

   The onboarding automation described in this issue is Bet C work, not Bet A or Bet B work.

5. **What does exist.** The wgmesh CLI already provides a largely self-contained onboarding
   path: `wgmesh init --secret` → `wgmesh signup --email` → `wgmesh join` → `wgmesh service
   add`. Quick-start documentation (`docs/`) and the 5-minute tutorial are the correct
   near-term vehicle for reducing time-to-value, not automated email sequences.

## Proposed Approach

Decline to implement the automation features in this issue at this time. The actionable
near-term investment in "time-to-value" is:

- **Keep the 5-minute tutorial accurate and runnable** (docs/plans, README quick-start).
- **Surface CLI-level health feedback** — the existing `wgmesh status` command is the
  correct foundation; extend it with connection-state reporting when the daemon is live.
- **Revisit this issue at Bet B** (Tenth paying customer, Q3) when self-serve signup
  launches and the team has real activation funnel data to inform which automation steps
  are highest-leverage.

No code or documentation changes are required to close this issue as wont-do.

## Affected Files

None.

## Test Strategy

N/A — no implementation deliverable.

## Estimated Complexity
low
