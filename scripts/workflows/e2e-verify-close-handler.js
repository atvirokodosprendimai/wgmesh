// scripts/workflows/e2e-verify-close-handler.js
//
// Handler for the `E2E Verify — Close Issue` workflow. Triggered from
// `.github/workflows/e2e-verify-close.yml` on the `workflow_run`
// `completed` event of the `E2E Verifier` workflow.
//
// Contract:
//   module.exports = async function handler({github, context, core})
//
// `github`  — Octokit-shaped client (must expose
//             .rest.issues.{get,update,addLabels,removeLabel,createComment},
//             .rest.pulls.get, .rest.repos.getCommit)
// `context` — Actions-shaped context. Reads .repo.{owner,repo} and
//             .payload.workflow_run.{conclusion, head_sha, html_url, pull_requests}
// `core`    — Actions-shaped logger. Reads .info(msg) and .warning(msg)
//
// Conclusion → action mapping:
//   - 'success'   → addLabels(['verified']),
//                   removeLabels(['awaiting-verification', 'awaiting-tests',
//                                 'e2e-failed', 'e2e-stalled']),
//                   close issue, comment with verifier run URL.
//   - 'failure'   → addLabels(['e2e-failed']),
//                   removeLabels(['awaiting-verification']),
//                   reopen issue iff currently closed,
//                   comment with verifier run URL + artifact link.
//   - other ('cancelled', 'timed_out', 'action_required', …) → log + exit.
//
// Modules are intentionally standalone (no shared imports with
// impl-merged-close-handler.js); the removeLabels-with-404-tolerance
// helper is duplicated here on purpose.
//
// Policy lives in `.github/workflows/e2e-verify-close.yml`'s top-of-file
// comment block. This file owns the implementation; do not duplicate the
// policy narrative here.

'use strict';

// removeLabels — best-effort removal. Mirrors the impl-merged-close-handler.js
// pattern (intentionally duplicated rather than imported — modules are
// standalone). 404 = label not present, expected when stale wasn't on the
// issue. Any other failure is logged and skipped.
async function removeLabels({github, context, core, issue_number, candidates}) {
  for (const stale of candidates) {
    try {
      await github.rest.issues.removeLabel({
        owner: context.repo.owner,
        repo: context.repo.repo,
        issue_number,
        name: stale
      });
    } catch (e) {
      if (e.status === 404) continue;
      core.warning(`Failed to remove ${stale} label: ${e.message}`);
    }
  }
}

// resolvePullRequest — pull the PR object from the workflow_run payload's
// `pull_requests` array. When that array is empty (e.g. the verifier was
// triggered from a fork or via workflow_dispatch with no pr_number),
// fall back to scanning the workflow run's commit message for an
// `Issue #N` reference. Returns either {prNumber, prTitle} or null.
async function resolvePullRequest({github, context, core, workflowRun}) {
  const prs = Array.isArray(workflowRun.pull_requests) ? workflowRun.pull_requests : [];
  if (prs.length > 0 && prs[0].number) {
    const prNumber = prs[0].number;
    try {
      const { data: pr } = await github.rest.pulls.get({
        owner: context.repo.owner,
        repo: context.repo.repo,
        pull_number: prNumber
      });
      return { prNumber, prTitle: pr.title || '' };
    } catch (e) {
      core.warning(`pulls.get failed for PR #${prNumber}: ${e.message}`);
      return { prNumber, prTitle: '' };
    }
  }

  // Fallback path: read the workflow run's head commit message and look
  // for `Issue #N` (the existing convention from impl-merged-close).
  if (!workflowRun.head_sha) {
    core.info('workflow_run.pull_requests is empty and head_sha is missing; cannot correlate.');
    return null;
  }
  try {
    const { data: commit } = await github.rest.repos.getCommit({
      owner: context.repo.owner,
      repo: context.repo.repo,
      ref: workflowRun.head_sha
    });
    const message = (commit.commit && commit.commit.message) || '';
    const issueMatch = message.match(/Issue #(\d+)/);
    if (issueMatch) {
      // Synthesize a PR-like shape. The downstream issue resolver only
      // needs prTitle to extract `Issue #N`; prNumber stays unset because
      // we don't actually have a PR.
      return { prNumber: null, prTitle: message.split('\n', 1)[0] };
    }
    core.info('Commit message does not reference an Issue #N; skipping.');
    return null;
  } catch (e) {
    core.warning(`repos.getCommit failed for ${workflowRun.head_sha}: ${e.message}`);
    return null;
  }
}

// extractIssueNumber — same regex shape as impl-merged-close-handler.js to
// stay compatible with the existing `impl: Issue #N - …` PR title
// convention. Returns the numeric issue or null.
function extractIssueNumber(prTitle) {
  if (!prTitle) return null;
  const m = prTitle.match(/Issue #(\d+)/);
  return m ? parseInt(m[1], 10) : null;
}

async function handler({github, context, core}) {
  const workflowRun = context.payload && context.payload.workflow_run;
  if (!workflowRun) {
    core.warning('No workflow_run payload; aborting.');
    return;
  }

  const conclusion = workflowRun.conclusion;
  const runUrl = workflowRun.html_url || '';
  core.info(`E2E Verifier conclusion: ${conclusion} (${runUrl})`);

  // Skip non-actionable conclusions early.
  if (conclusion !== 'success' && conclusion !== 'failure') {
    core.info(`Conclusion ${conclusion} is non-actionable; no label changes.`);
    return;
  }

  const resolved = await resolvePullRequest({github, context, core, workflowRun});
  if (!resolved) return;

  const issueNumber = extractIssueNumber(resolved.prTitle);
  if (!issueNumber) {
    core.info(`PR title "${resolved.prTitle}" does not contain Issue #N; skipping.`);
    return;
  }

  // Need the issue's current state to decide whether to reopen on failure.
  let issue;
  try {
    const result = await github.rest.issues.get({
      owner: context.repo.owner,
      repo: context.repo.repo,
      issue_number: issueNumber
    });
    issue = result.data;
  } catch (e) {
    core.warning(`issues.get failed for #${issueNumber}: ${e.message}`);
    return;
  }

  const prRef = resolved.prNumber ? `PR #${resolved.prNumber}` : `commit ${workflowRun.head_sha || 'unknown'}`;

  if (conclusion === 'success') {
    // SUCCESS: add `verified`, remove all in-flight labels, close the
    // issue, comment with verifier run URL.
    await github.rest.issues.addLabels({
      owner: context.repo.owner,
      repo: context.repo.repo,
      issue_number: issueNumber,
      labels: ['verified']
    });
    await removeLabels({
      github, context, core,
      issue_number: issueNumber,
      candidates: ['awaiting-verification', 'awaiting-tests', 'e2e-failed', 'e2e-stalled']
    });
    if (issue.state !== 'closed') {
      await github.rest.issues.update({
        owner: context.repo.owner,
        repo: context.repo.repo,
        issue_number: issueNumber,
        state: 'closed'
      });
    }
    const successBody = [
      `Verified: ${prRef} merged and the E2E Verifier confirmed the fix on the merge commit.`,
      ``,
      `**Verifier run:** ${runUrl}`,
      ``,
      `_Closed automatically by e2e-verify-close.yml. Policy lives in \`.github/workflows/e2e-verify-close.yml\`._`
    ].join('\n');
    await github.rest.issues.createComment({
      owner: context.repo.owner,
      repo: context.repo.repo,
      issue_number: issueNumber,
      body: successBody
    });
    return;
  }

  // FAILURE: add `e2e-failed`, remove `awaiting-verification`, reopen
  // the issue ONLY if currently closed, comment with run URL + artifact
  // hint.
  await github.rest.issues.addLabels({
    owner: context.repo.owner,
    repo: context.repo.repo,
    issue_number: issueNumber,
    labels: ['e2e-failed']
  });
  await removeLabels({
    github, context, core,
    issue_number: issueNumber,
    candidates: ['awaiting-verification']
  });
  if (issue.state === 'closed') {
    await github.rest.issues.update({
      owner: context.repo.owner,
      repo: context.repo.repo,
      issue_number: issueNumber,
      state: 'open'
    });
  }
  const failureBody = [
    `E2E Verifier reported a failure for ${prRef} on the merge commit. The fix did not survive the integration subset, so this issue stays open until a follow-up addresses the failure.`,
    ``,
    `**Verifier run:** ${runUrl}`,
    `**Artifacts:** check the "tier-N-logs" attachments on the run page above for trace.jsonl + tier-summary.md.`,
    ``,
    `_Reopened (if it was closed) by e2e-verify-close.yml. Re-running the verifier on the same SHA flips back to \`verified\` on success._`
  ].join('\n');
  await github.rest.issues.createComment({
    owner: context.repo.owner,
    repo: context.repo.repo,
    issue_number: issueNumber,
    body: failureBody
  });
}

module.exports = handler;
// Internal helpers exported for unit testing.
module.exports.extractIssueNumber = extractIssueNumber;
module.exports.resolvePullRequest = resolvePullRequest;
module.exports.removeLabels = removeLabels;
