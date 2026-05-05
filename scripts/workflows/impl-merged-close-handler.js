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
// The handler returns nothing on the happy paths and never throws on
// expected branches. Unexpected failures inside an octokit call propagate.
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
    if (Array.isArray(data) || !data.content) return null;
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

    const fromContent = new Set();
    core.info(`Content-diff fallback for ${f.filename} (patch=${!!f.patch}, fromPatch=${fromPatch.size}, status=${f.status})`);
    const headFuncs = await fetchFileFuncs({github, core, context, path: f.filename, ref: pr.head.sha});
    if (headFuncs === null) {
      indeterminateFiles.push(f.filename);
    } else if (f.status === 'added' || f.status === 'copied') {
      for (const fn of headFuncs) fromContent.add(fn);
    } else {
      const basePath = (f.status === 'renamed' && f.previous_filename)
        ? f.previous_filename
        : f.filename;
      const baseFuncs = await fetchFileFuncs({github, core, context, path: basePath, ref: pr.base.sha});
      if (baseFuncs === null) {
        indeterminateFiles.push(f.filename);
      } else {
        for (const fn of headFuncs) {
          if (!baseFuncs.has(fn)) fromContent.add(fn);
        }
      }
    }

    for (const fn of fromPatch) newTestFuncSet.add(fn);
    for (const fn of fromContent) newTestFuncSet.add(fn);
  }

  return { newTestFuncs: [...newTestFuncSet], indeterminateFiles };
}

// removeLabels — best-effort removal of a list of label names. Each remove
// is wrapped in a try/catch so a missing-label 404 doesn't abort the run.
// Order-sensitive callers should still call this BEFORE addLabels (see
// race-condition note in the workflow comment block).
async function removeLabels({github, context, core, issue_number, labelNames, candidates}) {
  for (const stale of candidates) {
    if (labelNames.includes(stale)) {
      try {
        await github.rest.issues.removeLabel({
          owner: context.repo.owner,
          repo: context.repo.repo,
          issue_number,
          name: stale
        });
      } catch (e) {
        core.warning(`Failed to remove ${stale} label: ${e.message}`);
      }
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

    // REMOVE stale labels before ADD to close the bypass race window
    // (verify-comment-close.yml could otherwise close on a "verified"
    // comment landed in the gap).
    await removeLabels({
      github, context, core,
      issue_number: issueNumber,
      labelNames,
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
      `PR #${pr.number} merged but the fix does not yet meet the regression-test policy for \`type: bug\` issues. The issue stays open until a follow-up PR adds a regression test.`,
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

  // Both gates passed → awaiting-verification.
  await removeLabels({
    github, context, core,
    issue_number: issueNumber,
    labelNames,
    candidates: ['copilot-triaging', 'needs-triage', 'awaiting-tests']
  });

  await github.rest.issues.addLabels({
    owner: context.repo.owner,
    repo: context.repo.repo,
    issue_number: issueNumber,
    labels: ['awaiting-verification']
  });

  const author = issue.user && issue.user.login ? `@${issue.user.login}` : 'reporter';
  const verifyBody = [
    `Implementation for this bug merged in PR #${pr.number}.`,
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
