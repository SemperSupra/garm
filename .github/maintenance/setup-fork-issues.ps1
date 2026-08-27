[CmdletBinding()]
param(
    [string]$Repo = "SemperSupra/garm",
    [string]$ExpectedUpstream = "cloudbase/garm",
    [string]$ProviderRepo = "SemperSupra/garm-provider-truenas",
    [string]$FoundryRepo = "SemperSupra/truenas-app-foundry",
    [string]$Branch = "chore/fork-issue-governance",
    [switch]$SkipPolicyIssue
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Require-Command {
    param([Parameter(Mandatory)][string]$Name)
    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        throw "Required command '$Name' was not found in PATH."
    }
}

function Invoke-Gh {
    param([Parameter(Mandatory)][string[]]$Arguments)
    & gh @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "gh command failed: gh $($Arguments -join ' ')"
    }
}

function Invoke-Git {
    param([Parameter(Mandatory)][string[]]$Arguments)
    & git @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "git command failed: git $($Arguments -join ' ')"
    }
}

Require-Command gh
Require-Command git

# Authentication and fork identity guardrails.
Invoke-Gh @("auth", "status")

$repoInfo = (gh api "repos/$Repo" | ConvertFrom-Json)
if (-not $repoInfo.fork) {
    throw "$Repo is not reported by GitHub as a fork; refusing to apply fork-specific issue governance."
}
if ($repoInfo.parent.full_name -ne $ExpectedUpstream) {
    throw "Unexpected upstream for $Repo. Expected '$ExpectedUpstream', found '$($repoInfo.parent.full_name)'."
}
$defaultBranch = [string]$repoInfo.default_branch

Write-Host "Repository: $Repo"
Write-Host "Upstream:   $ExpectedUpstream"
Write-Host "Default:    $defaultBranch"

# Enable Issues. This is safe to rerun.
Invoke-Gh @("repo", "edit", $Repo, "--enable-issues")

# Restrict issue creation to collaborators only.
# GitHub added repository issueCreationPolicy to GraphQL UpdateRepositoryInput in 2026.
$repoNodeId = (& gh api graphql `
    -f query='query($owner:String!,$name:String!){repository(owner:$owner,name:$name){id issueCreationPolicy}}' `
    -F owner="$($Repo.Split('/')[0])" `
    -F name="$($Repo.Split('/')[1])" `
    --jq '.data.repository.id').Trim()
if ($LASTEXITCODE -ne 0 -or -not $repoNodeId) {
    throw "Failed to resolve repository node ID for $Repo."
}

Invoke-Gh @(
    "api", "graphql",
    "-f", 'query=mutation($id:ID!){updateRepository(input:{repositoryId:$id,issueCreationPolicy:COLLABORATORS_ONLY}){repository{id issueCreationPolicy}}}',
    "-f", "id=$repoNodeId"
)

$policy = (& gh api graphql `
    -f query='query($owner:String!,$name:String!){repository(owner:$owner,name:$name){issueCreationPolicy}}' `
    -F owner="$($Repo.Split('/')[0])" `
    -F name="$($Repo.Split('/')[1])" `
    --jq '.data.repository.issueCreationPolicy').Trim()
if ($LASTEXITCODE -ne 0 -or $policy -ne "COLLABORATORS_ONLY") {
    throw "Issue creation policy verification failed for $Repo. Expected COLLABORATORS_ONLY, found '$policy'."
}
Write-Host "Issue creation policy: $policy"

# Orthogonal fork-specific taxonomy:
# - classification labels describe why the issue exists in the fork;
# - state labels describe its relationship with upstream.
$labels = @(
    @{ Name = "fork:upstream-bound";        Color = "1D76DB"; Description = "Fork issue tracks a defect/change intended for upstream GARM." },
    @{ Name = "fork:downstream-only";       Color = "5319E7"; Description = "Intentional SemperSupra-only delta; must state rationale and exit condition." },
    @{ Name = "fork:qualification";         Color = "0E8A16"; Description = "Exact candidate qualification, regression evidence, or promotion blocker." },
    @{ Name = "fork:needs-upstream-report"; Color = "FBCA04"; Description = "Confirmed upstream-relevant defect still needs an upstream issue/PR link." },
    @{ Name = "fork:blocked-upstream";      Color = "D93F0B"; Description = "Downstream work is waiting on an upstream decision, fix, or release." },
    @{ Name = "fork:upstream-fixed";        Color = "BFDADC"; Description = "Upstream fix exists; downstream should verify, sync, then retire local delta." },
    @{ Name = "fork:governance";            Color = "C5DEF5"; Description = "Fork maintenance policy, scope, or repository-process issue." },
    @{ Name = "type:bug";                   Color = "D73A4A"; Description = "Something is not working as intended." }
)

foreach ($label in $labels) {
    Invoke-Gh @(
        "label", "create", $label.Name,
        "--repo", $Repo,
        "--color", $label.Color,
        "--description", $label.Description,
        "--force"
    )
}

$tempRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("fork-issue-governance-" + [Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $tempRoot | Out-Null

try {
    Invoke-Gh @("repo", "clone", $Repo, $tempRoot, "--", "--filter=blob:none")
    Push-Location $tempRoot
    try {
        Invoke-Git @("fetch", "origin", $defaultBranch)
        Invoke-Git @("switch", "-C", $Branch, "origin/$defaultBranch")

        $templateDir = Join-Path $tempRoot ".github/ISSUE_TEMPLATE"
        New-Item -ItemType Directory -Force -Path $templateDir | Out-Null

        @"
blank_issues_enabled: false
contact_links:
  - name: Upstream GARM — external reports belong upstream
    url: https://github.com/$ExpectedUpstream
    about: This fork does not track externally submitted issues. Use upstream Issues if GitHub permits issue creation; otherwise use the upstream community channel documented in its README.
  - name: TrueNAS external-provider issues
    url: https://github.com/$ProviderRepo/issues
    about: TrueNAS Apps/Containers/VM provider implementation and runtime behavior belongs here.
  - name: GARM TrueNAS appliance / Foundry issues
    url: https://github.com/$FoundryRepo/issues
    about: Appliance assembly, GHCR image, source qualification, and TrueNAS App packaging belongs here.
"@ | Set-Content -Encoding utf8 (Join-Path $templateDir "config.yml")

        @"
name: Upstream-bound defect
description: Maintainer-only upstreaming tracker for a GARM defect this fork is actively carrying toward cloudbase/garm.
title: "[upstream] "
labels:
  - fork:upstream-bound
  - type:bug
body:
  - type: markdown
    attributes:
      value: |
        **Maintainer/upstreaming queue only. Issue creation is restricted to repository collaborators.**
        Use this only when the issue affects the SemperSupra GARM fork itself.
        Search cloudbase/garm first. Do not duplicate an existing upstream issue; link it instead.
        Do not put credentials, tokens, private infrastructure details, or security-sensitive exploit details in a public issue.
  - type: input
    id: affected_sha
    attributes:
      label: Affected SemperSupra/garm SHA
      description: Exact 40-character commit SHA when known.
      placeholder: 0123456789abcdef0123456789abcdef01234567
    validations:
      required: true
  - type: dropdown
    id: upstream_status
    attributes:
      label: Upstream status
      options:
        - Existing upstream issue/PR
        - Needs upstream report
        - Upstream declined / downstream decision needed
    validations:
      required: true
  - type: input
    id: upstream_link
    attributes:
      label: Upstream issue or PR
      description: Link cloudbase/garm issue/PR if one already exists; otherwise write "none yet".
      placeholder: https://github.com/cloudbase/garm/issues/123
    validations:
      required: true
  - type: textarea
    id: problem
    attributes:
      label: Problem
      description: What is wrong, and why does it matter to the fork?
    validations:
      required: true
  - type: textarea
    id: reproducer
    attributes:
      label: Minimal reproducer / evidence
      description: Include the smallest public-safe reproducer, logs, or test evidence.
    validations:
      required: true
  - type: textarea
    id: proposed_fix
    attributes:
      label: Proposed downstream patch
      description: Describe the smallest upstream-oriented patch, if known.
  - type: textarea
    id: exit
    attributes:
      label: Exit condition
      description: Define when the fork issue can close (for example upstream merge + fork sync + regression PASS).
    validations:
      required: true
"@ | Set-Content -Encoding utf8 (Join-Path $templateDir "upstream-defect.yml")

        @"
name: Intentional downstream delta
description: Maintainer-only tracker for the rare temporary fork delta that cannot yet be upstreamed.
title: "[downstream] "
labels:
  - fork:downstream-only
body:
  - type: markdown
    attributes:
      value: |
        **Maintainer/upstreaming queue only. Issue creation is restricted to repository collaborators.**
        Downstream deltas should be rare. Prefer upstream-compatible configuration or an upstream patch when practical.
        TrueNAS-provider and appliance-specific work belongs in their owning repositories, not here.
  - type: input
    id: base
    attributes:
      label: Upstream/base identity
      description: Upstream tag/SHA or SemperSupra base SHA this delta is built on.
    validations:
      required: true
  - type: textarea
    id: rationale
    attributes:
      label: Why must this live in the fork?
      description: Explain why configuration, an extension point, or an upstream change is not sufficient.
    validations:
      required: true
  - type: textarea
    id: compatibility
    attributes:
      label: Compatibility and rebase risk
      description: What upstream areas does this touch and what must be revalidated when syncing?
    validations:
      required: true
  - type: textarea
    id: exit
    attributes:
      label: Exit / retirement condition
      description: State how this delta is eventually removed, upstreamed, or deliberately retained.
    validations:
      required: true
"@ | Set-Content -Encoding utf8 (Join-Path $templateDir "downstream-delta.yml")

        @"
name: Qualification / promotion blocker
description: Maintainer-only qualification for an exact GARM candidate or upstreaming blocker.
title: "[qualification] "
labels:
  - fork:qualification
body:
  - type: input
    id: candidate
    attributes:
      label: Exact candidate SHA
      placeholder: 0123456789abcdef0123456789abcdef01234567
    validations:
      required: true
  - type: textarea
    id: scope
    attributes:
      label: Qualification scope
      description: State exactly what behavior or patch is under qualification.
    validations:
      required: true
  - type: textarea
    id: evidence
    attributes:
      label: Required evidence
      description: Tests, public CI runs, build artifacts, red/green regression proof, etc.
    validations:
      required: true
  - type: textarea
    id: nonclaims
    attributes:
      label: Explicit non-claims
      description: State what this qualification does NOT prove (for example TrueNAS HIL, provider functionality, or production readiness).
    validations:
      required: true
  - type: textarea
    id: acceptance
    attributes:
      label: Acceptance / promotion criteria
    validations:
      required: true
"@ | Set-Content -Encoding utf8 (Join-Path $templateDir "qualification.yml")

        $policyPath = Join-Path $tempRoot ".github/FORK_ISSUE_POLICY.md"
        @"
# Fork issue policy

$Repo is an upstream-oriented fork of $ExpectedUpstream.

Issues are enabled here solely as a **maintainer/upstreaming work queue** for changes this fork is actively carrying, qualifying, or preparing to send upstream.

This repository does **not** operate a public downstream issue tracker:
- issue creation is restricted by GitHub to repository collaborators;
- external bug reports and feature requests belong in $ExpectedUpstream;
- external TrueNAS-provider or appliance reports belong in their owning repositories.

The restriction is intentional: fork Issues exist only for maintainer work that is actively being carried, qualified, or prepared for upstream submission.

## In scope

- **fork:upstream-bound** — a defect/change affecting the fork that is being investigated or carried locally and is intended for upstream;
- **fork:downstream-only** — an intentional SemperSupra GARM delta that cannot reasonably be expressed as configuration or an external extension;
- **fork:qualification** — exact candidate qualification, regression evidence, or a blocker to promoting a SemperSupra GARM SHA.

## Routing

Search $ExpectedUpstream before opening an upstream-bound issue. Link an existing upstream issue/PR rather than duplicating it. If no upstream report exists yet, add fork:needs-upstream-report.

TrueNAS external-provider implementation belongs in $ProviderRepo.
GARM TrueNAS appliance assembly, GHCR packaging, and Foundry qualification belong in $FoundryRepo.

Misfiled issues can be transferred between SemperSupra repositories when appropriate. They cannot be transferred directly into the Cloudbase organization; create/link the upstream issue or PR instead.

## External submissions

The fork issue tracker is not an intake queue for the public. GitHub's repository-level issue creation policy is set to `COLLABORATORS_ONLY`.

For this organization repository, only users with sufficient repository access can create issues. External users cannot create fork issues. If an external report elsewhere results in a patch that this fork must carry, a maintainer creates the local upstreaming/qualification issue.

## Lifecycle

An upstream-bound issue should close after the upstream fix is accepted (or otherwise resolved), the fork is synchronized/rebased as appropriate, and the relevant regression/qualification evidence passes.

A downstream-only issue must record why the delta exists, its compatibility/rebase risk, and an explicit retirement or retention condition.

A qualification issue must identify an exact candidate SHA, required evidence, acceptance criteria, and explicit non-claims.

## Safety

Never place credentials, tokens, private infrastructure details, or sensitive security exploit material in public fork issues.
"@ | Set-Content -Encoding utf8 $policyPath

        Invoke-Git @("add", ".github/ISSUE_TEMPLATE", ".github/FORK_ISSUE_POLICY.md")
        $status = (& git status --porcelain)
        if ($LASTEXITCODE -ne 0) { throw "git status failed" }

        $prUrl = $null
        if ($status) {
            Invoke-Git @("commit", "-m", "chore: add fork issue governance")
            Invoke-Git @("push", "--force-with-lease", "-u", "origin", $Branch)

            $existingPr = @(
                gh pr list --repo $Repo --state open --head $Branch --base $defaultBranch --json number,url |
                    ConvertFrom-Json
            )
            if ($existingPr.Count -gt 0) {
                $prUrl = [string]$existingPr[0].url
                Write-Host "Existing governance PR: $prUrl"
            } else {
                $prBody = @"
Establishes fork-specific issue governance for $Repo.

- keeps the fork explicitly upstream-oriented toward $ExpectedUpstream rather than mirroring the upstream tracker;
- defines Issues as a collaborator-only maintainer/upstreaming work queue;
- adds structured issue forms for upstream-bound defects, rare intentional downstream deltas, and exact-candidate qualification work;
- disables blank issues and routes users to upstream GARM or the owning TrueNAS repositories;
- adds a durable fork issue policy with routing, lifecycle, and safety rules.

This PR intentionally does not create product-specific GARM/TrueNAS issues in the fork. External reports are not tracked here; if an outside report results in a fork-carried patch, a maintainer creates the local upstreaming/qualification issue.
"@
                $prUrl = (& gh pr create --repo $Repo --base $defaultBranch --head $Branch `
                    --title "chore: add fork issue governance" --body $prBody)
                if ($LASTEXITCODE -ne 0) { throw "gh pr create failed" }
                Write-Host "Created governance PR: $prUrl"
            }
        } else {
            Write-Host "No governance file changes were needed."
        }

        if (-not $SkipPolicyIssue) {
            $policyTitle = "Fork issue policy and routing"
            $existingIssues = @(
                gh issue list --repo $Repo --state all --limit 100 `
                    --json number,title |
                    ConvertFrom-Json |
                    Where-Object { $_.title -eq $policyTitle }
            )

            if ($existingIssues.Count -gt 0) {
                $policyNumber = [int]$existingIssues[0].number
                Write-Host "Existing policy issue: #$policyNumber"
            } else {
                $policyBody = @"
This pinned issue is the routing index for $Repo.

**This fork does not track externally submitted issues.** Issue creation is restricted to repository collaborators. External GARM bugs and feature requests belong with https://github.com/$ExpectedUpstream. Use upstream Issues when GitHub permits issue creation; otherwise use the upstream community/support channel documented in its README.

The fork issue tracker is intentionally narrow, restricted to collaborators, and exists only for maintainer/upstreaming work:

- fork:upstream-bound: a GARM defect/change actively carried or investigated in this fork and intended for upstream;
- fork:downstream-only: a deliberate SemperSupra GARM delta with rationale and an exit/retention condition;
- fork:qualification: exact candidate regression/qualification work.

Search $ExpectedUpstream first and link existing upstream issues/PRs rather than duplicating them.

TrueNAS provider implementation belongs in https://github.com/$ProviderRepo/issues.
GARM TrueNAS appliance/Foundry work belongs in https://github.com/$FoundryRepo/issues.

The durable policy/templates are introduced by the fork-governance PR. After that PR merges, the structured issue forms become available on the default branch.
"@
                $policyUrl = (& gh issue create --repo $Repo --title $policyTitle `
                    --body $policyBody --label "fork:governance")
                if ($LASTEXITCODE -ne 0) { throw "gh issue create failed" }

                $policyNumberText = ($policyUrl -split "/")[-1]
                $policyNumber = [int]$policyNumberText
                Write-Host "Created policy issue: $policyUrl"
            }

            $isPinnedText = (& gh issue view $policyNumber --repo $Repo --json isPinned --jq ".isPinned").Trim()
            if ($LASTEXITCODE -ne 0) { throw "gh issue view failed" }
            $isPinned = $isPinnedText -eq "true"
            if (-not $isPinned) {
                Invoke-Gh @("issue", "pin", "$policyNumber", "--repo", $Repo)
            }

            # Keep the routing issue from turning into a support/discussion thread.
            # Maintainers can still unlock it deliberately if policy needs to change.
            Invoke-Gh @("issue", "lock", "$policyNumber", "--repo", $Repo, "--reason", "resolved")
        }

        Write-Host ""
        Write-Host "Fork issue governance bootstrap complete."
        Write-Host "Important: issue templates/forms become active only after the governance PR is merged into '$defaultBranch'. The collaborators-only creation policy is applied immediately."
    }
    finally {
        Pop-Location
    }
}
finally {
    if (Test-Path $tempRoot) {
        Remove-Item -Recurse -Force $tempRoot
    }
}
