# Specification: Issue #714

## Classification
bug

## Problem Analysis

The Copilot spec generation workflow is experiencing a critical bottleneck that is blocking 7 PRs from proceeding. Based on the current codebase analysis, the bottleneck appears to be in the `goose-triage.yml` workflow, which is responsible for automatically generating specification documents when issues are labeled with `needs-triage`.

Current symptoms:
1. 7 PRs are blocked and cannot proceed to implementation
2. The spec generation process is timing out or failing
3. The workflow uses a 40-minute timeout (`timeout-minutes: 40`) for Goose CLI execution
4. The Goose CLI is configured to use model `GLM-5.1` via Anthropic provider
5. The workflow reads issue context, runs Goose with a recipe, validates output, and creates PRs

The root cause is likely one or more of:
- The GLM-5.1 model is too slow for spec generation, frequently hitting or approaching the 40-minute timeout
- The recipe prompt (`wgmesh-triage-spec.yaml`) is too verbose, causing excessive token usage and slow generation
- Multiple concurrent triage workflows are competing for API rate limits
- The sanitise script (`company/scripts/sanitise.sh`) is adding overhead or failing intermittently
- The workflow does not have proper retry logic for transient failures
- There is no visibility into which step is causing the bottleneck (setup, Goose run, validation, or PR creation)

The workflow currently tracks duration and lines output but does not expose this information in a way that allows debugging of slow runs. The lack of per-step timing metrics makes it difficult to identify whether the bottleneck is in:
- Goose CLI installation
- Repository checkout and analysis
- LLM inference (most likely)
- Spec validation
- Git operations and PR creation

## Proposed Approach

Implement observability and optimization improvements to the spec generation workflow to reduce the bottleneck and unblock the 7 stuck PRs. The approach focuses on three areas:

1. **Add granular timing metrics** to identify exactly which step is slow
2. **Optimize the recipe prompt** to reduce token usage and generation time
3. **Add retry logic and fallback** for transient failures
4. **Add concurrency control** to prevent multiple workflows from competing for resources

The proposed tasks will instrument the workflow with per-step timing, add a fallback faster model, optimize the prompt template, and add circuit-breaker logic to prevent cascading failures. This will unblock the current PRs and prevent future bottlenecks.

## Implementation Tasks

### Task 1: Add granular timing metrics to goose-triage.yml

Modify `.github/workflows/goose-triage.yml` to track timing for each major step:

1. Add timing tracking after "Checkout repository" step:
   ```yaml
   - name: Track checkout time
     run: echo "checkout_time=$(date +%s)" >> $GITHUB_OUTPUT
   ```

2. Add timing tracking after "Install Goose CLI" step:
   ```yaml
   - name: Track install time
     run: echo "install_time=$(date +%s)" >> $GITHUB_OUTPUT
   ```

3. Add timing tracking after "Run Goose with recipe" step (already has goose_duration):
   ```yaml
   - name: Track goose run time
     run: echo "goose_run_time=${{ steps.goose.outputs.goose_duration }}" >> $GITHUB_OUTPUT
   ```

4. Add timing tracking after "Sanitise spec" step:
   ```yaml
   - name: Track sanitise time
     run: echo "sanitise_time=$(date +%s)" >> $GITHUB_OUTPUT
   ```

5. Add a summary step before "Commit and push spec":
   ```yaml
   - name: Output timing summary
     run: |
       echo "## Timing Summary" >> $GITHUB_STEP_SUMMARY
       echo "- Checkout: ${{ steps.checkout.outputs.checkout_time }}s" >> $GITHUB_STEP_SUMMARY
       echo "- Install: ${{ steps.install.outputs.install_time }}s" >> $GITHUB_STEP_SUMMARY
       echo "- Goose run: ${{ steps.goose.outputs.goose_duration }}s" >> $GITHUB_STEP_SUMMARY
       echo "- Sanitise: ${{ steps.sanitise.outputs.sanitise_time }}s" >> $GITHUB_STEP_SUMMARY
       echo "- Total: $(( $(date +%s) - ${{ steps.start.outputs.start_time }} ))s" >> $GITHUB_STEP_SUMMARY
   ```

### Task 2: Add fallback model configuration

Create environment variable for model selection with fallback:

1. Add new environment variables in `.github/workflows/goose-triage.yml`:
   ```yaml
   env:
     GOOSE_MODEL_PRIMARY: "GLM-5.1"
     GOOSE_MODEL_FALLBACK: "claude-3-5-sonnet-20241022"
     GOOSE_MODEL_TIMEOUT: "1800"  # 30 minutes
   ```

2. Modify the Goose run step to check timeout and retry with fallback:
   ```yaml
   - name: Run Goose with recipe (primary)
     id: goose-primary
     continue-on-error: true
     run: |
       SPEC_FILE="specs/issue-${ISSUE_NUM}-spec.md"
       timeout $GOOSE_MODEL_TIMEOUT goose run \
         --recipe .github/goose-recipes/wgmesh-triage-spec.yaml \
         --params "spec_file=$SPEC_FILE" \
         --no-session \
         2>&1 | tee /tmp/goose-output.log
     if [ ${{ steps.goose-primary.outcome }} == 'failure' ]; then
       echo "Primary model timed out or failed, trying fallback..."
       goose run \
         --recipe .github/goose-recipes/wgmesh-triage-spec-fallback.yaml \
         --params "spec_file=$SPEC_FILE" \
         --no-session \
         2>&1 | tee /tmp/goose-output-fallback.log
     fi
   ```

### Task 3: Create optimized fallback recipe

Create `.github/goose-recipes/wgmesh-triage-spec-fallback.yaml` with a shorter, more focused prompt:

```yaml
---
version: "1.0.0"
title: "wgmesh Triage Spec (Fast)"
description: >-
  Fast spec generation for wgmesh using optimized prompt

parameters:
  - key: spec_file
    input_type: string
    requirement: required
    description: "Path to the specification file (specs/issue-N-spec.md)"

extensions:
  - type: builtin
    name: developer

prompt: |
  You are writing a spec ONLY for wgmesh (Go 1.25 WireGuard mesh).

  Read /tmp/issue-context.md then create {{ spec_file }} with:
  - ## Classification (single word: bug/feature/chore)
  - ## Problem Analysis (2-4 sentences)
  - ## Proposed Approach (2-4 sentences)
  - ## Implementation Tasks (numbered, concrete steps)
  - ## Affected Files (list)
  - ## Acceptance Criteria (bullet list)
  - ## Out of Scope (bullet list)

  NO code. NO git commands. PR created by workflow.
  Read relevant source files before writing.

settings:
  goose_provider: "anthropic"
  goose_model: "claude-3-5-sonnet-20241022"
```

### Task 4: Add concurrency limit

Add concurrency control to prevent multiple triage workflows from running simultaneously:

1. Add concurrency block at the job level in `.github/workflows/goose-triage.yml`:
   ```yaml
   jobs:
     triage:
       concurrency:
         group: goose-triage
         cancel-in-progress: false
       if: github.event.label.name == 'needs-triage'
       runs-on: ubuntu-latest
       timeout-minutes: 40
   ```

### Task 5: Add workflow failure notifications

Add notification step when workflow fails:

```yaml
- name: Notify on failure
  if: failure()
  uses: actions/github-script@v7
  with:
    github-token: ${{ secrets.GITHUB_TOKEN }}
    script: |
      const issueNum = process.env.ISSUE_NUM;
      await github.rest.issues.createComment({
        owner: context.repo.owner,
        repo: context.repo.repo,
        issue_number: parseInt(issueNum),
        body: `⚠️ Spec generation failed. Check workflow logs for details.`
      });
```

### Task 6: Optimize main recipe prompt

Simplify `.github/goose-recipes/wgmesh-triage-spec.yaml` prompt:

1. Reduce the CRITICAL section from verbose instructions to concise bullet points
2. Condense the specification steps from 7 detailed steps to 3 essential steps
3. Remove redundant warnings about PR creation (already stated once)
4. Keep required sections and template requirements but reduce explanation length

Target: Reduce prompt token count by 40-50% to speed up generation.

### Task 7: Add debug mode for stuck workflows

Add a debug workflow input for manually re-running with verbose output:

```yaml
on:
  workflow_dispatch:
    inputs:
      issue_number:
        description: 'Issue number to triage'
        required: true
      debug:
        description: 'Enable debug mode'
        type: boolean
        default: false
```

Modify Goose run step to add `--debug` flag when debug is enabled.

## Affected Files

- `.github/workflows/goose-triage.yml` - Add timing metrics, concurrency, fallback logic, notifications
- `.github/goose-recipes/wgmesh-triage-spec.yaml` - Optimize prompt length
- `.github/goose-recipes/wgmesh-triage-spec-fallback.yaml` - Create new fallback recipe
- `company/scripts/sanitise.sh` - Review for optimization opportunities (optional)

## Acceptance Criteria

1. Workflow completes spec generation in under 15 minutes in 90% of cases
2. Timing metrics are visible in GitHub Actions step summary for every run
3. Fallback to faster model activates when primary model exceeds 30 minutes
4. Concurrency limit prevents more than 2 triage workflows from running simultaneously
5. Failed workflows automatically post a comment to the issue with next steps
6. Existing 7 blocked PRs are cleared within 24 hours of deployment
7. Sanitise script continues to pass (no regressions in security filtering)

## Out of Scope

- Modifying the spec-auto-approve.yml workflow (separate concern)
- Changing the spec template structure (that's governed by other specs)
- Modifying the goose-build.yml implementation workflow
- Changing the sanitise.sh script security patterns
- Modifying the spec validation logic in spec-auto-approve.yml
- Any changes to the actual spec content structure (Classification, Problem Analysis, etc.)
