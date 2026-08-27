# Fork issue policy

`SemperSupra/garm` is an upstream-oriented fork of `cloudbase/garm`.

Issues are enabled here solely as a **maintainer/upstreaming work queue** for changes this fork is actively carrying, qualifying, or preparing to send upstream.

This repository does **not** operate a public downstream issue tracker:
- issue creation is restricted to repository collaborators;
- external GARM bug reports and feature requests belong with `cloudbase/garm`;
- TrueNAS-provider and GARM TrueNAS appliance issues belong in their owning repositories.

The restriction is intentional: fork Issues exist only for maintainer work that is actively being carried, qualified, or prepared for upstream submission.

## In scope

- **`fork:upstream-bound`** — a defect or change affecting the fork that is being investigated or carried locally and is intended for upstream;
- **`fork:downstream-only`** — a rare, intentional SemperSupra GARM delta that cannot reasonably be expressed as configuration or an external extension and has an explicit retirement or retention condition;
- **`fork:qualification`** — exact candidate qualification, regression evidence, or a blocker to promoting a SemperSupra GARM SHA.

## Routing

Search `cloudbase/garm` before opening an upstream-bound issue. Link an existing upstream issue or PR rather than duplicating it. If no upstream report exists yet, add `fork:needs-upstream-report`.

TrueNAS external-provider implementation belongs in `SemperSupra/garm-provider-truenas`.

GARM TrueNAS appliance assembly, GHCR packaging, source qualification, and Foundry work belong in `SemperSupra/truenas-app-foundry`.

Misfiled issues can be transferred between SemperSupra repositories when appropriate. They cannot be transferred directly into the Cloudbase organization; create or link the upstream issue or PR instead.

## External submissions

The fork issue tracker is not an intake queue for the public. GitHub's repository-level issue creation policy is set to `COLLABORATORS_ONLY`.

For this organization repository, only users with sufficient repository access can create issues. External users cannot create fork issues. If an external report elsewhere results in a patch that this fork must carry, a maintainer creates the local upstreaming or qualification issue.

## Lifecycle

An upstream-bound issue should close after the upstream fix is accepted or otherwise resolved, the fork is synchronized or rebased as appropriate, and the relevant regression or qualification evidence passes.

A downstream-only issue must record why the delta exists, its compatibility or rebase risk, and an explicit retirement or retention condition.

A qualification issue must identify an exact candidate SHA, required evidence, acceptance criteria, and explicit non-claims.

## Safety

Never place credentials, tokens, private infrastructure details, or sensitive security exploit material in public fork issues.
