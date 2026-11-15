// scripts/workflows/auto-merge.test.js
//
// Tests for the trust gate in .github/workflows/auto-merge.yml.
// Runtime: node --test scripts/workflows/auto-merge.test.js
//
// The workflow logic is an inline actions/github-script body, so this test
// pulls that body out of the YAML and runs it with a mocked Octokit client.
// What it runs is the text that GitHub runs, not a copy of it.

'use strict';

const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const path = require('node:path');

const WORKFLOW = path.join(__dirname, '..', '..', '.github', 'workflows', 'auto-merge.yml');
const AsyncFunction = Object.getPrototypeOf(async function () {}).constructor;

function loadScript() {
  const lines = fs.readFileSync(WORKFLOW, 'utf8').split('\n');
  const start = lines.findIndex(l => /^\s+script: \|\s*$/.test(l));
  assert.ok(start >= 0, 'script: | block not found in auto-merge.yml');
  const indent = lines[start + 1].match(/^ */)[0].length;
  const body = [];
  for (let i = start + 1; i < lines.length; i++) {
    const l = lines[i];
    if (l.trim() !== '' && l.match(/^ */)[0].length < indent) break;
    body.push(l.slice(indent));
  }
  return body.join('\n');
}

const SCRIPT = loadScript();

function pr(overrides = {}) {
  return {
    number: 7,
    draft: false,
    state: 'open',
    mergeable: true,
    base: { ref: 'main' },
    head: { sha: 'headsha1', repo: { full_name: 'o/r' } },
    labels: [],
    user: { login: 'someone' },
    created_at: '2026-01-01T00:00:00Z',
    ...overrides,
  };
}

// run executes the workflow script against one open PR and reports what it did.
async function run({ pull, reviews = [] }) {
  const calls = { merge: [], approve: [], warnings: [] };
  const github = {
    rest: {
      pulls: {
        list: async () => ({ data: [{ number: pull.number, draft: false }] }),
        get: async () => ({ data: pull }),
        updateBranch: async () => ({}),
        listReviews: async () => ({ data: reviews }),
        listReviewComments: async () => ({ data: [] }),
        merge: async args => { calls.merge.push(args); return {}; },
      },
      checks: {
        listForRef: async () => ({ data: { check_runs: [{ name: 'build', status: 'completed', conclusion: 'success' }] } }),
      },
      issues: { addLabels: async () => ({}), removeLabel: async () => ({}) },
    },
    graphql: async () => ({ repository: { pullRequest: { reviewThreads: { nodes: [] } } } }),
  };
  const context = { eventName: 'workflow_dispatch', repo: { owner: 'o', repo: 'r' }, payload: {} };
  const core = { info: () => {}, warning: m => calls.warnings.push(m) };
  const fakeFs = { writeFileSync: () => {} };

  const realFetch = globalThis.fetch;
  globalThis.fetch = async (url, opts) => {
    if (String(url).endsWith('/reviews') && opts && opts.method === 'POST') {
      calls.approve.push(JSON.parse(opts.body));
    }
    return { ok: true, status: 200, json: async () => ({ workflow_runs: [] }), text: async () => '' };
  };
  const realEnv = { token: process.env.ACTIONS_BOT_TOKEN, run: process.env.GITHUB_RUN_ID };
  process.env.ACTIONS_BOT_TOKEN = 'test-token';
  process.env.GITHUB_RUN_ID = '1';
  try {
    const fn = new AsyncFunction('github', 'context', 'core', 'require', SCRIPT);
    await fn(github, context, core, name => (name === 'fs' ? fakeFs : require(name)));
  } finally {
    globalThis.fetch = realFetch;
    if (realEnv.token === undefined) delete process.env.ACTIONS_BOT_TOKEN; else process.env.ACTIONS_BOT_TOKEN = realEnv.token;
    if (realEnv.run === undefined) delete process.env.GITHUB_RUN_ID; else process.env.GITHUB_RUN_ID = realEnv.run;
  }
  return calls;
}

test('a fork PR with green checks is neither approved nor merged', async () => {
  const calls = await run({ pull: pr({ head: { sha: 'forksha', repo: { full_name: 'evil/wgmesh' } } }) });
  assert.strictEqual(calls.merge.length, 0, 'fork PR was merged');
  assert.strictEqual(calls.approve.length, 0, 'fork PR was approved');
  assert.ok(calls.warnings.some(w => /evil\/wgmesh/.test(w)), 'skip was not reported');
});

test('a PR whose fork was deleted (head.repo null) is not merged', async () => {
  const calls = await run({ pull: pr({ head: { sha: 'x', repo: null } }) });
  assert.strictEqual(calls.merge.length, 0);
  assert.strictEqual(calls.approve.length, 0);
});

test('a same-repo PR is approved and merged, both bound to the head sha', async () => {
  const calls = await run({ pull: pr() });
  assert.strictEqual(calls.approve.length, 1, 'expected one approval');
  assert.strictEqual(calls.approve[0].commit_id, 'headsha1');
  assert.strictEqual(calls.merge.length, 1, 'expected one merge');
  assert.strictEqual(calls.merge[0].sha, 'headsha1');
});

test('an approval of an older commit does not count: the PR is approved again', async () => {
  const calls = await run({ pull: pr(), reviews: [{ state: 'APPROVED', commit_id: 'oldsha' }] });
  assert.strictEqual(calls.approve.length, 1, 'stale approval was accepted');
  assert.strictEqual(calls.approve[0].commit_id, 'headsha1');
});

test('an approval of the current commit is reused', async () => {
  const calls = await run({ pull: pr(), reviews: [{ state: 'APPROVED', commit_id: 'headsha1' }] });
  assert.strictEqual(calls.approve.length, 0, 'approved again although already approved at this commit');
  assert.strictEqual(calls.merge.length, 1);
});
