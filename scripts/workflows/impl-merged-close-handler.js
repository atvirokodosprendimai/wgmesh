// scripts/workflows/impl-merged-close-handler.js
//
// Handler for the `Implementation Merged — Close Issue` workflow. Extracted
// from inline `actions/github-script` so the logic is unit-testable and
// readable without scrolling through YAML.
//
// Contract:
//   module.exports = async function handler({github, context, core})
//
// `github`  — Octokit-shaped client (must expose .rest.issues.{get,update,addLabels,removeLabel,createComment}, .rest.pulls.listFiles, .rest.repos.getContent, and a top-level .paginate that follows pagination)
// `context` — Actions-shaped context. Reads .repo.{owner,repo} and .payload.pull_request.{number,title,body,head.sha,base.sha}
// `core`    — Actions-shaped logger. Reads .info(msg) and .warning(msg)
//
// The handler returns nothing on the happy paths. It is best-effort with
// respect to two specific octokit calls: `repos.getContent` and
// `issues.removeLabel` failures are caught and treated as "indeterminate"
// or "no-op" respectively (file may be too large / removed / 404 on
// concurrently-removed label). All other octokit failures propagate.
//
// Policy lives in `.github/workflows/impl-merged-close.yml`'s top-of-file
// comment block. This file owns the implementation; do not duplicate the
// policy narrative here.

'use strict';

const TEST_FUNC_REGEX_ADDED = /^\+func\s+(Test[A-Z][A-Za-z0-9_]*)\s*\(\s*t\s+\*testing\.T\s*\)/gm;
const TEST_FUNC_REGEX_ANY = /^\s*func\s+(Test[A-Z][A-Za-z0-9_]*)\s*\(\s*t\s+\*testing\.T\s*\)/gm;

const REPRO_REGEX = /(?:^|\n)#{1,6}\s+(?:steps to reproduce|reproduction|how to reproduce|repro)\b[^\n]*\n([\s\S]*?)(?:\n#{1,6}\s|\n*$)/i;

const STOP_WORDS = new Set([
  'about', 'after', 'again', 'against', 'because', 'before', 'being',
  'below', 'between', 'could', 'doing', 'during', 'each', 'every',
  'fixed', 'first', 'further', 'happens', 'having', 'here', 'into',
  'issue', 'itself', 'might', 'more', 'most', 'much', 'never',
  'normal', 'other', 'over', 'point', 'really', 'reproduce',
  'reproduction', 'same', 'should', 'since', 'some', 'specific',
  'still', 'such', 'than', 'that', 'their', 'them', 'then', 'there',
  'these', 'they', 'this', 'those', 'through', 'until', 'very',
  'want', 'were', 'what', 'when', 'where', 'which', 'while', 'with',
  'would', 'your', 'yourself', 'shell', 'logs', 'environment',
  'response', 'expected', 'behavior', 'description'
]);

// extractRepoTokens — pull lowercase alphanumeric tokens of length ≥ 5 from
// the issue body's "Steps to Reproduce" (or whole body fallback). Returns
// a deduped array.
function extractRepoTokens(body) {
  if (!body) return [];
  const reproMatch = body.match(REPRO_REGEX);
  const reproText = reproMatch ? reproMatch[1] : body;
  const tokens = (reproText.toLowerCase().match(/[a-z][a-z0-9_]{4,}/g) || [])
    .filter(t => !STOP_WORDS.has(t));
  return [...new Set(tokens)];
}

// labelNamesOf — issue.labels can be ['name'] or [{name: 'foo'}, ...]. Normalize.
function labelNamesOf(issue) {
  return (issue.labels || []).map(l => typeof l === 'string' ? l : l.name);
}

function isBug(labelNames) {
  return labelNames.some(n => n === 'type: bug' || n === 'bug');
}

// fetchFileFuncs — return a Set of ALL Test func names declared in the file
// at <ref>. Returns null on getContent failure (file too large, binary,
// transient API error, missing path).
async function fetchFileFuncs({github, core, context, path, ref}) {
  try {
    const { data } = await github.rest.repos.getContent({
      owner: context.repo.owner,
      repo: context.repo.repo,
      path,
      ref
    });
    // Empty files return content: "" (base64 of empty), which decodes to
    // an empty string — not an error. Distinguish "missing/non-string" from
    // "empty" so empty test files produce an empty Set rather than null.
    if (Array.isArray(data) || typeof data.content !== 'string') return null;
    const decoded = Buffer.from(data.content, data.encoding || 'base64').toString('utf-8');
    const funcs = new Set();
    TEST_FUNC_REGEX_ANY.lastIndex = 0;
    let m;
    while ((m = TEST_FUNC_REGEX_ANY.exec(decoded)) !== null) {
      funcs.add(m[1]);
    }
    return funcs;
  } catch (e) {
    core.warning(`getContent failed for ${path}@${ref}: ${e.message}`);
    return null;
  }
}

// detectNewTestFuncs — for each *_test.go file in the PR diff, run BOTH
// patch parsing AND content diff (always-on, even when patch parsing finds
// matches, to defeat truncated-patch false negatives). Union the results.
// Returns { newTestFuncs: string[], indeterminateFiles: string[] }.
async function detectNewTestFuncs({github, context, core, pr}) {
  const prFiles = await github.paginate(github.rest.pulls.listFiles, {
    owner: context.repo.owner,
    repo: context.repo.repo,
    pull_number: pr.number,
    per_page: 100
  });

  const newTestFuncSet = new Set();
  const indeterminateFiles = [];

  for (const f of prFiles) {
    if (!f.filename.endsWith('_test.go')) continue;
    if (f.status === 'removed') continue;

    const fromPatch = new Set();
    if (f.patch) {
      TEST_FUNC_REGEX_ADDED.lastIndex = 0;
      let m;
      while ((m = TEST_FUNC_REGEX_ADDED.exec(f.patch)) !== null) {
        fromPatch.add(m[1]);
      }
    }

    // Conditional content fallback. The earlier always-on version was
    // expensive (1-2 extra REST calls per test file even when patch was
    // perfectly readable) and pushed against rate limits on busy PRs.
    // Now we run it only when patch parsing missed something — either
    // patch is absent (binary/too-large diff) OR patch parsing produced
    // no matches (could be no test changes OR could be a truncated patch
    // that hid the new test funcs below the cutoff). When fromPatch.size > 0
    // we trust the patch result and skip the fallback.
    const fromContent = new Set();
    const needContentDiff = !f.patch || fromPatch.size === 0;
    let headFuncs = null;
    if (needContentDiff) {
      core.info(`Content-diff fallback for ${f.filename} (patch=${!!f.patch}, fromPatch=${fromPatch.size}, status=${f.status})`);
      headFuncs = await fetchFileFuncs({github, core, context, path: f.filename, ref: pr.head.sha});
    }
    if (needContentDiff) {
      if (headFuncs === null) {
        // Mark indeterminate ONLY when both patch parsing AND content diff
        // are unavailable. If patch parsing yielded results (handled below)
        // the file isn't truly indeterminate — we just couldn't double-check.
        if (fromPatch.size === 0) indeterminateFiles.push(f.filename);
      } else if (f.status === 'added' || f.status === 'copied') {
        for (const fn of headFuncs) fromContent.add(fn);
      } else {
        const basePath = (f.status === 'renamed' && f.previous_filename)
          ? f.previous_filename
          : f.filename;
        const baseFuncs = await fetchFileFuncs({github, core, context, path: basePath, ref: pr.base.sha});
        if (baseFuncs === null) {
          if (fromPatch.size === 0) indeterminateFiles.push(f.filename);
        } else {
          for (const fn of headFuncs) {
            if (!baseFuncs.has(fn)) fromContent.add(fn);
          }
        }
      }
    }

    for (const fn of fromPatch) newTestFuncSet.add(fn);
    for (const fn of fromContent) newTestFuncSet.add(fn);
  }

  return { newTestFuncs: [...newTestFuncSet], indeterminateFiles };
}

// removeLabels — best-effort removal. Tries each candidate unconditionally
// and swallows 404 (label not present). Earlier version gated on the
// initial labelNames snapshot; that missed labels added concurrently
// between issue.get and the removal call (race the workflow exists to
// avoid). Unconditional + 404-tolerant is more robust.
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
      // 404 = label not present, expected when stale wasn't on the issue.
      if (e.status === 404) continue;
      core.warning(`Failed to remove ${stale} label: ${e.message}`);
    }
  }
}

async function handler({github, context, core}) {
  const pr = context.payload.pull_request;
  const issueMatch = pr.title.match(/Issue #(\d+)/);
  if (!issueMatch) return;

  const issueNumber = parseInt(issueMatch[1], 10);

  const { data: issue } = await github.rest.issues.get({
    owner: context.repo.owner,
    repo: context.repo.repo,
    issue_number: issueNumber
  });

  const labelNames = labelNamesOf(issue);

  if (!isBug(labelNames)) {
    await github.rest.issues.update({
      owner: context.repo.owner,
      repo: context.repo.repo,
      issue_number: issueNumber,
      state: 'closed'
    });
    await github.rest.issues.createComment({
      owner: context.repo.owner,
      repo: context.repo.repo,
      issue_number: issueNumber,
      body: `Resolved by PR #${pr.number}. Implementation merged to main.`
    });
    return;
  }

  core.info(`Issue #${issueNumber} carries bug label; running test/keyword gates.`);

  // Reopen-on-bypass guard. GitHub's PR-body keyword auto-close (`Closes #N`,
  // `Fixes #N`, `Resolves #N`) closes the issue ~2s after PR merge — BEFORE
  // this workflow's `pull_request: closed` event has dispatched. By the
  // time the handler runs, the issue is already in state=closed with
  // state_reason=completed, bypassing the L2/L3 gate entirely.
  //
  // If the issue is closed AND lacks the gate's lifecycle labels
  // (awaiting-tests / awaiting-verification), assume native bypass and
  // reopen so the gate can do its job. The reporter sees a brief flap
  // (closed → reopened) which is the price of the bypass-detection.
  // Skip when one of the gate labels IS present (means a previous run of
  // the gate already classified the issue, and a separate close happened
  // legitimately — e.g., reporter said "verified" → verify-comment-close.yml).
  const wasBypassed = issue.state === 'closed' &&
    !labelNames.includes('awaiting-tests') &&
    !labelNames.includes('awaiting-verification');
  if (wasBypassed) {
    core.warning(`Issue #${issueNumber} was already closed (likely by a GitHub native 'Closes #N' / 'Fixes #N' / 'Resolves #N' keyword in PR #${pr.number}'s body). Reopening to run the bug-gate.`);
    await github.rest.issues.update({
      owner: context.repo.owner,
      repo: context.repo.repo,
      issue_number: issueNumber,
      state: 'open',
      state_reason: 'reopened'
    });
  }

  const { newTestFuncs, indeterminateFiles } = await detectNewTestFuncs({github, context, core, pr});

  const hasNewTest = newTestFuncs.length > 0;
  const l2Passes = hasNewTest;

  const reproTokens = extractRepoTokens(issue.body);
  const haystack = (newTestFuncs.join(' ') + ' ' + (pr.body || '')).toLowerCase();
  const matchedTokens = reproTokens.filter(t => haystack.includes(t));
  const hasKeywordMatch = matchedTokens.length > 0;

  if (!l2Passes || !hasKeywordMatch) {
    const failedGates = [];
    if (!l2Passes) failedGates.push('L2 — no new `func TestXxx(t *testing.T)` declaration in any `*_test.go` file in this PR diff');
    if (!hasKeywordMatch) {
      if (reproTokens.length === 0) {
        failedGates.push('L3 — issue body has no extractable reproduction tokens (consider adding a "Steps to Reproduce" section)');
      } else {
        failedGates.push(`L3 — none of the reproduction tokens (\`${reproTokens.slice(0, 8).join('`, `')}\`...) appear in the new test names or PR description`);
      }
    }

    // FAILURE PATH ordering: REMOVE stale labels before ADD.
    // If a previous PR landed the issue in awaiting-verification, leaving
    // that label present alongside awaiting-tests would let
    // verify-comment-close.yml close on a "verified" comment in the gap
    // window — bypassing the test gate. Removing first closes the gap.
    await removeLabels({
      github, context, core,
      issue_number: issueNumber,
      candidates: ['awaiting-verification', 'copilot-triaging', 'needs-triage']
    });

    await github.rest.issues.addLabels({
      owner: context.repo.owner,
      repo: context.repo.repo,
      issue_number: issueNumber,
      labels: ['awaiting-tests']
    });

    const author = issue.user && issue.user.login ? `@${issue.user.login}` : 'reporter';
    const blockBody = [
      `PR #${pr.number} merged but the fix does not yet meet the regression-test policy for \`type: bug\` issues. The issue stays open until a follow-up PR adds a regression test.${wasBypassed ? `\n\n_Note: this issue was auto-closed by GitHub's native \`Closes #N\` keyword resolution. The bug gate has reopened it so the test policy can be enforced. Future impl PRs should use \`Implements #N\` instead of \`Closes #N\` to avoid the flap._` : ''}`,
      ``,
      `**Failed gates:**`,
      ``,
      ...failedGates.map(g => `- ${g}`),
      ``,
      `**New test funcs found in this PR:** ${newTestFuncs.length === 0 ? 'none' : newTestFuncs.map(n => '`' + n + '`').join(', ')}`,
      ...(indeterminateFiles.length > 0 ? [`**Could not determine new test funcs for:** ${indeterminateFiles.map(f => '`' + f + '`').join(', ')} (file content unavailable from getContent — likely too large for the API, binary, transient API error, or insufficient token permissions). These files do NOT count toward L2; if you believe they contain a new regression test, a maintainer can override by removing \`awaiting-tests\` and adding \`awaiting-verification\`.`] : []),
      `**Reproduction tokens extracted:** ${reproTokens.length === 0 ? 'none' : reproTokens.slice(0, 8).map(t => '`' + t + '`').join(', ') + (reproTokens.length > 8 ? '…' : '')}`,
      ``,
      `**Required action:** ship a follow-up PR that adds a \`func Test...(t *testing.T)\` exercising the reproduction. ${author}, the issue will return to verification once that lands.`,
      ``,
      `_Policy lives in \`.github/workflows/impl-merged-close.yml\`. The wgmesh#540 incident motivated this gate — a partial fix shipped + auto-closed without a regression test, and the bug returned 6 days later._`
    ].join('\n');

    await github.rest.issues.createComment({
      owner: context.repo.owner,
      repo: context.repo.repo,
      issue_number: issueNumber,
      body: blockBody
    });
    return;
  }

  // SUCCESS PATH ordering: ADD awaiting-verification FIRST, then remove
  // stale labels. Inverse of the failure-path ordering. Reasoning:
  // verify-comment-close.yml only closes when a "verified" comment lands
  // while awaiting-verification is present. If we removed the stale
  // labels first, there'd be a window where neither awaiting-tests nor
  // awaiting-verification was on the issue — a fast "verified" comment
  // in that gap would be ignored permanently. addLabels is idempotent,
  // so adding even when already present is fine.
  await github.rest.issues.addLabels({
    owner: context.repo.owner,
    repo: context.repo.repo,
    issue_number: issueNumber,
    labels: ['awaiting-verification']
  });

  await removeLabels({
    github, context, core,
    issue_number: issueNumber,
    candidates: ['copilot-triaging', 'needs-triage', 'awaiting-tests']
  });

  const author = issue.user && issue.user.login ? `@${issue.user.login}` : 'reporter';
  const verifyBody = [
    `Implementation for this bug merged in PR #${pr.number}.${wasBypassed ? ` (Note: GitHub auto-closed this issue via \`Closes #N\` keyword; the bug gate reopened it because the reporter still needs to verify the fix.)` : ''}`,
    ``,
    `**Test gate passed.** New test funcs: ${newTestFuncs.map(n => '`' + n + '`').join(', ')}.`,
    `**Reproduction-keyword match:** \`${matchedTokens.slice(0, 5).join('`, `')}\`.`,
    ``,
    `${author}, please verify the fix against your original reproduction and reply with one of:`,
    ``,
    `- **\`verified\`** / **\`confirmed\`** / **\`fixed\`** if the bug no longer reproduces — the issue will be closed.`,
    `- **\`still broken\`** / **\`not fixed\`** with details if the bug still reproduces — the issue stays open and we'll ship a follow-up.`,
    ``,
    `_Auto-close is disabled for \`type: bug\` issues. See \`.github/workflows/impl-merged-close.yml\` for the policy._`
  ].join('\n');

  await github.rest.issues.createComment({
    owner: context.repo.owner,
    repo: context.repo.repo,
    issue_number: issueNumber,
    body: verifyBody
  });
}

module.exports = handler;
// Internal helpers exported for unit testing.
module.exports.extractRepoTokens = extractRepoTokens;
module.exports.labelNamesOf = labelNamesOf;
module.exports.isBug = isBug;
module.exports.detectNewTestFuncs = detectNewTestFuncs;
module.exports.STOP_WORDS = STOP_WORDS;
