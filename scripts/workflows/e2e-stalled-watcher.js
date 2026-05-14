// scripts/workflows/e2e-stalled-watcher.js
//
// Cron-driven watcher for awaiting-verification issues that never received
// an e2e-verifier conclusion. Runs every 30 min via the parent workflow.
//
// Contract:
//   module.exports = async function handler({github, context, core})
//
// Behavior:
//   1. Lists open issues carrying the `awaiting-verification` label.
//   2. For each, computes the "freshness" timestamp:
//        - We rely on issue.updated_at as a coarse proxy. The "ideal"
//          signal is the most recent label-event timestamp for
//          `awaiting-verification`, which would require fetching
//          `issues.listEventsForTimeline` per issue. That's an extra
//          API call per issue per cron tick, vs `updated_at` which
//          comes free in the list response. Tradeoff documented:
//          comments on the issue also bump updated_at, which means a
//          chatty reporter can keep updated_at "fresh" past the SLA
//          window and we will under-flag. Acceptable for now (the
//          watcher's job is to surface, not to enforce); revisit if
//          the operator dashboard ever shows missed flags.
//   3. If `now - updated_at > 6h` AND labels do NOT include any of
//      `verified`, `e2e-failed`, `e2e-stalled`, add `e2e-stalled`.
//   4. Idempotent — re-runs add the label only if missing (addLabels is
//      idempotent on GitHub's side, but we skip pre-emptively to keep
//      the audit log clean).
//   5. Logs detected count to $GITHUB_STEP_SUMMARY when GITHUB_STEP_SUMMARY
//      env is set (only in workflow runs; tests skip the write).

'use strict';

const fs = require('node:fs');

const STALL_BUDGET_MS = 6 * 60 * 60 * 1000; // 6 hours

const TERMINAL_LABELS = new Set(['verified', 'e2e-failed', 'e2e-stalled']);

// labelNamesOf — issue.labels can be ['name'] or [{name: 'foo'}]. Normalize.
function labelNamesOf(issue) {
  return (issue.labels || []).map(l => typeof l === 'string' ? l : (l && l.name) || '');
}

// shouldFlag — pure decision: given an issue's label set + updated_at vs now,
// should we add `e2e-stalled`?
function shouldFlag({labels, updatedAt, now, budgetMs = STALL_BUDGET_MS}) {
  if (!updatedAt) return false;
  const updatedMs = Date.parse(updatedAt);
  if (Number.isNaN(updatedMs)) return false;
  if (now - updatedMs <= budgetMs) return false;
  if (!labels.includes('awaiting-verification')) return false;
  for (const t of TERMINAL_LABELS) {
    if (labels.includes(t)) return false;
  }
  return true;
}

async function handler({github, context, core, nowMs}) {
  const now = typeof nowMs === 'number' ? nowMs : Date.now();

  // listForRepo paginates open issues with the awaiting-verification label.
  // Result shape: each item has .number, .labels, .updated_at, .pull_request
  // (set when the "issue" is actually a PR — we filter those out).
  const issues = await github.paginate(github.rest.issues.listForRepo, {
    owner: context.repo.owner,
    repo: context.repo.repo,
    state: 'open',
    labels: 'awaiting-verification',
    per_page: 100
  });

  let stalledCount = 0;
  const stalledNumbers = [];

  for (const issue of issues) {
    if (issue.pull_request) continue; // PRs share the issue API; skip.
    const labels = labelNamesOf(issue);
    if (!shouldFlag({labels, updatedAt: issue.updated_at, now})) continue;

    try {
      await github.rest.issues.addLabels({
        owner: context.repo.owner,
        repo: context.repo.repo,
        issue_number: issue.number,
        labels: ['e2e-stalled']
      });
      stalledCount += 1;
      stalledNumbers.push(issue.number);
      core.info(`Flagged #${issue.number} as e2e-stalled (updated_at=${issue.updated_at}).`);
    } catch (e) {
      core.warning(`Failed to add e2e-stalled to #${issue.number}: ${e.message}`);
    }
  }

  // Step summary (best-effort; tests run without GITHUB_STEP_SUMMARY).
  const summaryPath = process.env.GITHUB_STEP_SUMMARY;
  if (summaryPath) {
    try {
      const lines = [
        `## e2e-stalled-watcher`,
        ``,
        `Detected **${stalledCount}** stalled \`awaiting-verification\` issue(s) past the 6h budget.`,
        ``
      ];
      if (stalledNumbers.length > 0) {
        lines.push(`Flagged: ${stalledNumbers.map(n => `#${n}`).join(', ')}`);
        lines.push('');
      }
      fs.appendFileSync(summaryPath, lines.join('\n'));
    } catch (e) {
      core.warning(`Failed to write step summary: ${e.message}`);
    }
  }

  return { stalledCount, stalledNumbers };
}

module.exports = handler;
// Internal helpers exported for unit testing.
module.exports.shouldFlag = shouldFlag;
module.exports.labelNamesOf = labelNamesOf;
module.exports.STALL_BUDGET_MS = STALL_BUDGET_MS;
module.exports.TERMINAL_LABELS = TERMINAL_LABELS;
