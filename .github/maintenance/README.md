# Fork maintenance tooling

This directory contains repository-local maintenance tooling for the upstream-oriented `SemperSupra/garm` fork.

## `setup-fork-issues.ps1`

Reproduces the fork Issues governance configuration:

- verifies `SemperSupra/garm` is a fork of `cloudbase/garm`;
- enables Issues and restricts issue creation to collaborators;
- creates/updates the `fork:*` label taxonomy;
- generates the structured issue forms and fork issue policy through a branch/PR;
- creates/reuses and locks the pinned routing issue.

Run from any working directory with an authenticated GitHub CLI and Git available:

```powershell
pwsh -NoProfile -ExecutionPolicy Bypass -File .github/maintenance/setup-fork-issues.ps1
```

The script is intended for maintainer/upstreaming governance only. It does not configure GARM runtime behavior.
