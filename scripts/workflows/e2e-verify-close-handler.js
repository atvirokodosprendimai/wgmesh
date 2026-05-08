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
//             .rest.issues.{get,update,addLabels,removeLabel,listComments,createComment},
//             .rest.pulls.get, .rest.repos.getCommit)
// `context` — Actions-shaped context. Reads .repo.{owner,repo} and
//             .payload.workflow_run.{conclusion, head_sha, html_url, pull_requests}
// `core`    — Actions-shaped logger. Reads .info(msg) and .warning(msg)
//
// Conclusion → action mapping:
//   - 'success'   → addLabels(['verified']),
//                   removeLabels(['awaiting-verification',
//                                 'e2e-failed', 'e2e-stalled']),
//                   close issue, comment with verifier run URL.
//                   NOTE: `awaiting-tests` is owned by the L4 gate in
//                   impl-merged-close-handler.js and is not cleared here.
//   - 'failure'   → addLabels(['e2e-failed']),
//                   removeLabels(['awaiting-verification', 'verified',
//                                 'e2e-stalled']),
//                   reopen issue iff currently closed AND issue was
//                   verifier-controlled (had awaiting-verification /
//                   verified / e2e-failed) at handler entry,
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

async function hasCommentForRun({github, context, core, issue_number, runUrl}) {
  if (!runUrl) return false;
  try {
    const { data: comments } = await github.rest.issues.listComments({
      owner: context.repo.owner,
      repo: context.repo.repo,
      issue_number,
      per_page: 30,
      sort: 'created',
      direction: 'desc'
    });
    return (comments || []).some(c => typeof c.body === 'string' && c.body.includes(runUrl));
  } catch (e) {
    core.warning(`issues.listComments failed for #${issue_number}: ${e.message}`);
    return false;
  }
}

async function createRunCommentOnce({github, context, core, issue_number, runUrl, body}) {
  const alreadyPosted = await hasCommentForRun({github, context, core, issue_number, runUrl});
  if (alreadyPosted) {
    core.info(`Skipping duplicate verifier comment for #${issue_number}: ${runUrl}`);
    return;
  }
  await github.rest.issues.createComment({
    owner: context.repo.owner,
    repo: context.repo.repo,
    issue_number,
    body
  });
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
    // Round-6 fix: only accept references that start a line, optionally
    // preceded by a conventional-commit prefix (`impl:`, `fix(daemon):`,
    // etc.). Mid-paragraph mentions like "see Issue #42 for context" or
    // "addresses Issue #123 by …" no longer match. The substring
    // fallback was dropped because it was permissive enough to mis-
    // associate when a commit body mentioned a different issue without
    // any structural anchor. When no anchored match exists, the verifier
    // run is silently skipped rather than risk flipping the wrong
    // issue's labels.
    //
    // Accepted line shapes (anywhere in the commit message):
    //   `Issue #123 — fix`
    //   `impl: Issue #556 - relay flap fix`
    //   `fix(daemon): Issue #444 - panic on rotate`
    const issueMatch = message.match(/^(?:[a-z]+(?:\([\w-]+\))?: )?Issue #(\d+)/m);
    if (issueMatch) {
      // Synthesize a prTitle that surfaces the Issue #N reference even
      // when it lives in the commit body (e.g., merge commits whose
      // first line is "Merge pull request #N from..."), so downstream
      // extractIssueNumber() doesn't silently miss the linkage.
      return { prNumber: null, prTitle: `Issue #${issueMatch[1]}` };
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

  // Capture verifier-controlled-state snapshot BEFORE we mutate labels
  // below. The issue is verifier-controlled if any of the workflow's
  // markers are present — including `verified` (already passed once)
  // and `e2e-failed` (already failed once). This lets a re-run of the
  // verifier on the same SHA correctly reopen a previously-verified
  // issue when the rerun fails. If none of the markers are present,
  // the issue is owned by something else (e.g., reporter close via
  // verify-comment-close.yml) and we must not reopen.
  const VERIFIER_CONTROLLED_LABELS = ['awaiting-verification', 'verified', 'e2e-failed'];
  const wasVerifierControlled = (issue.labels || []).some(l => {
    const name = typeof l === 'string' ? l : (l && l.name);
    return VERIFIER_CONTROLLED_LABELS.includes(name);
  });

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
    // Round-5 fix: do NOT remove `awaiting-tests` here. The L4 gate in
    // impl-merged-close-handler.js owns awaiting-tests — it stays on the
    // issue until an integration test ships. If awaiting-tests is still
    // present at verifier-success time, that's a state mismatch the L4
    // gate must resolve (or, more likely, awaiting-tests was already
    // removed by L4 when the issue advanced to awaiting-verification).
    // Removing it here bypasses the documented gate behavior.
    await removeLabels({
      github, context, core,
      issue_number: issueNumber,
      candidates: ['awaiting-verification', 'e2e-failed', 'e2e-stalled']
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
    await createRunCommentOnce({github, context, core, issue_number: issueNumber, runUrl, body: successBody});
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
    // Round-4 fix: also remove `verified` so a previously-verified issue
    // that fails on re-run does not end up labeled both `verified` AND
    // `e2e-failed` (contradictory state confuses pulse + downstream
    // automation).
    // Round-5 fix: also clear `e2e-stalled`. The stalled-watcher applies
    // it when the verifier exceeds 6h with no conclusion; if that watcher
    // fires AND a later verifier run finally returns failure, the issue
    // would otherwise carry both `e2e-stalled` AND `e2e-failed` —
    // contradictory and ambiguous for downstream automation.
    // Note: NOT clearing `awaiting-tests` here for the same reason as
    // the success path — the L4 gate owns it.
    candidates: ['awaiting-verification', 'verified', 'e2e-stalled']
  });
  if (issue.state === 'closed' && wasVerifierControlled) {
    await github.rest.issues.update({
      owner: context.repo.owner,
      repo: context.repo.repo,
      issue_number: issueNumber,
      state: 'open'
    });
  } else if (issue.state === 'closed') {
    core.info(
      `Issue #${issueNumber} is closed without any verifier-controlled label ` +
      `(awaiting-verification/verified/e2e-failed); skipping reopen to avoid ` +
      `racing a reporter-driven close.`
    );
  }
  const failureBody = [
    `E2E Verifier reported a failure for ${prRef} on the merge commit. The fix did not survive the integration subset, so this issue stays open until a follow-up addresses the failure.`,
    ``,
    `**Verifier run:** ${runUrl}`,
    `**Artifacts:** check the "tier-N-logs" attachments on the run page above for trace.jsonl + tier-summary.md.`,
    ``,
    `_Reopened (if it was closed) by e2e-verify-close.yml. Re-running the verifier on the same SHA flips back to \`verified\` on success._`
  ].join('\n');
  await createRunCommentOnce({github, context, core, issue_number: issueNumber, runUrl, body: failureBody});
}

module.exports = handler;
// Internal helpers exported for unit testing.
module.exports.extractIssueNumber = extractIssueNumber;
module.exports.resolvePullRequest = resolvePullRequest;
module.exports.removeLabels = removeLabels;
module.exports.hasCommentForRun = hasCommentForRun;
