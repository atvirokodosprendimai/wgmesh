// scripts/workflows/e2e-stalled-watcher.test.js
//
// Unit tests for e2e-stalled-watcher.js.
// Runtime: node --test scripts/workflows/e2e-stalled-watcher.test.js

'use strict';

const test = require('node:test');
const assert = require('node:assert');

const handler = require('./e2e-stalled-watcher.js');
const { shouldFlag, labelNamesOf, STALL_BUDGET_MS } = handler;

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

function mockCore() {
  return { info: () => {}, warning: () => {} };
}

const HOUR = 60 * 60 * 1000;

function isoMinus(ms, hoursAgo) {
  return new Date(ms - hoursAgo * HOUR).toISOString();
}

function makeGithub({ issues = [], recordCalls = [], addLabelsImpl } = {}) {
  return {
    paginate: async (fn, params) => {
      if (fn && fn._kind === 'listForRepo') return issues;
      throw new Error('unexpected paginate target');
    },
    rest: {
      issues: {
        listForRepo: Object.assign(async () => ({ data: issues }), { _kind: 'listForRepo' }),
        addLabels: addLabelsImpl || (async (params) => { recordCalls.push({ kind: 'addLabels', params }); })
      }
    }
  };
}

function ctx() {
  return { repo: { owner: 'O', repo: 'R' } };
}

// ---------------------------------------------------------------------------
// shouldFlag — pure decision
// ---------------------------------------------------------------------------

test('shouldFlag — fresh (5h ago) issue is not flagged', () => {
  const now = Date.now();
  assert.strictEqual(shouldFlag({
    labels: ['awaiting-verification'],
    updatedAt: isoMinus(now, 5),
    now
  }), false);
});

test('shouldFlag — stale (7h ago) awaiting-verification issue is flagged', () => {
  const now = Date.now();
  assert.strictEqual(shouldFlag({
    labels: ['awaiting-verification'],
    updatedAt: isoMinus(now, 7),
    now
  }), true);
});

test('shouldFlag — stale + already e2e-stalled is not flagged again', () => {
  const now = Date.now();
  assert.strictEqual(shouldFlag({
    labels: ['awaiting-verification', 'e2e-stalled'],
    updatedAt: isoMinus(now, 7),
    now
  }), false);
});

test('shouldFlag — stale + already e2e-failed is not flagged', () => {
  const now = Date.now();
  assert.strictEqual(shouldFlag({
    labels: ['awaiting-verification', 'e2e-failed'],
    updatedAt: isoMinus(now, 8),
    now
  }), false);
});

test('shouldFlag — stale + already verified is not flagged (race window)', () => {
  const now = Date.now();
  assert.strictEqual(shouldFlag({
    labels: ['awaiting-verification', 'verified'],
    updatedAt: isoMinus(now, 12),
    now
  }), false);
});

test('shouldFlag — issue without awaiting-verification label is not flagged', () => {
  const now = Date.now();
  assert.strictEqual(shouldFlag({
    labels: ['type: bug'],
    updatedAt: isoMinus(now, 24),
    now
  }), false);
});

test('shouldFlag — missing updatedAt is not flagged', () => {
  const now = Date.now();
  assert.strictEqual(shouldFlag({
    labels: ['awaiting-verification'],
    updatedAt: null,
    now
  }), false);
});

test('shouldFlag — invalid updatedAt is not flagged', () => {
  const now = Date.now();
  assert.strictEqual(shouldFlag({
    labels: ['awaiting-verification'],
    updatedAt: 'not-a-date',
    now
  }), false);
});

test('shouldFlag — exactly at 6h boundary is not flagged (strictly greater than budget)', () => {
  const now = Date.now();
  assert.strictEqual(shouldFlag({
    labels: ['awaiting-verification'],
    updatedAt: new Date(now - STALL_BUDGET_MS).toISOString(),
    now
  }), false);
});

// ---------------------------------------------------------------------------
// labelNamesOf
// ---------------------------------------------------------------------------

test('labelNamesOf — handles object form', () => {
  assert.deepStrictEqual(labelNamesOf({ labels: [{ name: 'a' }, { name: 'b' }] }), ['a', 'b']);
});

test('labelNamesOf — handles string form', () => {
  assert.deepStrictEqual(labelNamesOf({ labels: ['a', 'b'] }), ['a', 'b']);
});

test('labelNamesOf — missing labels returns []', () => {
  assert.deepStrictEqual(labelNamesOf({}), []);
});

// ---------------------------------------------------------------------------
// handler — integration over a list of issues
// ---------------------------------------------------------------------------

test('handler — single stalled issue → exactly one addLabels call', async () => {
  const recordCalls = [];
  const now = Date.now();
  const github = makeGithub({
    issues: [
      { number: 556, labels: [{ name: 'awaiting-verification' }], updated_at: isoMinus(now, 7) }
    ],
    recordCalls
  });

  const result = await handler({ github, context: ctx(), core: mockCore(), nowMs: now });

  assert.strictEqual(result.stalledCount, 1);
  assert.deepStrictEqual(result.stalledNumbers, [556]);
  const adds = recordCalls.filter(c => c.kind === 'addLabels');
  assert.strictEqual(adds.length, 1);
  assert.deepStrictEqual(adds[0].params.labels, ['e2e-stalled']);
});

test('handler — three issues, two stalled and one fresh → exactly two addLabels calls', async () => {
  const recordCalls = [];
  const now = Date.now();
  const github = makeGithub({
    issues: [
      { number: 1, labels: [{ name: 'awaiting-verification' }], updated_at: isoMinus(now, 7) },
      { number: 2, labels: [{ name: 'awaiting-verification' }], updated_at: isoMinus(now, 9) },
      { number: 3, labels: [{ name: 'awaiting-verification' }], updated_at: isoMinus(now, 1) }
    ],
    recordCalls
  });

  const result = await handler({ github, context: ctx(), core: mockCore(), nowMs: now });

  assert.strictEqual(result.stalledCount, 2);
  assert.deepStrictEqual(result.stalledNumbers.sort((a, b) => a - b), [1, 2]);
});

test('handler — issue already carrying e2e-stalled is not re-flagged', async () => {
  const recordCalls = [];
  const now = Date.now();
  const github = makeGithub({
    issues: [
      {
        number: 7,
        labels: [{ name: 'awaiting-verification' }, { name: 'e2e-stalled' }],
        updated_at: isoMinus(now, 12)
      }
    ],
    recordCalls
  });

  const result = await handler({ github, context: ctx(), core: mockCore(), nowMs: now });

  assert.strictEqual(result.stalledCount, 0);
  assert.strictEqual(recordCalls.filter(c => c.kind === 'addLabels').length, 0);
});

test('handler — PRs in the listForRepo response are skipped', async () => {
  const recordCalls = [];
  const now = Date.now();
  const github = makeGithub({
    issues: [
      // GitHub's listForRepo conflates issues + PRs; pull_request being set
      // means this is actually a PR. The watcher must ignore those.
      {
        number: 564,
        labels: [{ name: 'awaiting-verification' }],
        updated_at: isoMinus(now, 12),
        pull_request: { url: 'https://example.com/pulls/564' }
      },
      { number: 556, labels: [{ name: 'awaiting-verification' }], updated_at: isoMinus(now, 12) }
    ],
    recordCalls
  });

  const result = await handler({ github, context: ctx(), core: mockCore(), nowMs: now });

  assert.deepStrictEqual(result.stalledNumbers, [556]);
});

test('handler — addLabels failure on one issue does not abort the rest', async () => {
  const recordCalls = [];
  const now = Date.now();
  const github = makeGithub({
    issues: [
      { number: 1, labels: [{ name: 'awaiting-verification' }], updated_at: isoMinus(now, 7) },
      { number: 2, labels: [{ name: 'awaiting-verification' }], updated_at: isoMinus(now, 8) }
    ],
    addLabelsImpl: async (params) => {
      if (params.issue_number === 1) throw new Error('transient');
      recordCalls.push({ kind: 'addLabels', params });
    }
  });

  const result = await handler({ github, context: ctx(), core: mockCore(), nowMs: now });

  // Only #2 succeeds, but the loop continues past #1's failure.
  assert.deepStrictEqual(result.stalledNumbers, [2]);
});
