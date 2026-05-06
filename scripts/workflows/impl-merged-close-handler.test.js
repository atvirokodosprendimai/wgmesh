// scripts/workflows/impl-merged-close-handler.test.js
//
// Unit tests for impl-merged-close-handler.js.
// Runtime: node --test scripts/workflows/impl-merged-close-handler.test.js
//
// No external deps. Mocks the Octokit-shaped client + Actions context.

'use strict';

const test = require('node:test');
const assert = require('node:assert');

const handler = require('./impl-merged-close-handler.js');
const { extractRepoTokens, labelNamesOf, isBug, detectNewTestFuncs } = handler;

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

function mockCore() {
  return {
    info: () => {},
    warning: () => {},
    _calls: []
  };
}

// makeGithub builds an Octokit-shaped mock. `issuesData` maps issue_number to issue object.
// `prFiles` is the listFiles return; `contentByRefPath` maps "ref:path" → base64 content.
function makeGithub({ issuesData = {}, prFiles = [], contentByRefPath = {}, recordCalls = [] } = {}) {
  return {
    paginate: async (fn, params) => {
      // The handler calls github.paginate(github.rest.pulls.listFiles, {...}).
      // Our mock just returns the canned prFiles list.
      if (fn && fn._kind === 'listFiles') return prFiles;
      throw new Error('unexpected paginate target');
    },
    rest: {
      issues: {
        get: async ({ issue_number }) => {
          const issue = issuesData[issue_number];
          if (!issue) throw new Error(`issue ${issue_number} not found`);
          return { data: issue };
        },
        update: async (params) => { recordCalls.push({ kind: 'update', params }); },
        addLabels: async (params) => { recordCalls.push({ kind: 'addLabels', params }); },
        removeLabel: async (params) => { recordCalls.push({ kind: 'removeLabel', params }); },
        createComment: async (params) => { recordCalls.push({ kind: 'createComment', params }); }
      },
      pulls: {
        listFiles: Object.assign(async () => ({ data: prFiles }), { _kind: 'listFiles' })
      },
      repos: {
        getContent: async ({ ref, path }) => {
          const key = `${ref}:${path}`;
          const content = contentByRefPath[key];
          if (content === undefined) {
            const e = new Error(`no content at ${key}`);
            e.status = 404;
            throw e;
          }
          return {
            data: {
              content: Buffer.from(content, 'utf-8').toString('base64'),
              encoding: 'base64'
            }
          };
        }
      }
    }
  };
}

function makeContext({ owner = 'O', repo = 'R', pr = {} } = {}) {
  return {
    repo: { owner, repo },
    payload: { pull_request: { number: 1, title: '', body: '', head: { sha: 'h' }, base: { sha: 'b' }, ...pr } }
  };
}

// ---------------------------------------------------------------------------
// extractRepoTokens
// ---------------------------------------------------------------------------

test('extractRepoTokens — finds tokens in Steps to Reproduce section', () => {
  const body = `### Bug Description

something is wrong

### Steps to Reproduce

run wgmesh join --token 123 change --token to 1234

### Expected Behavior

ip should remain same`;

  const tokens = extractRepoTokens(body);
  assert.ok(tokens.includes('wgmesh'), 'should include wgmesh');
  assert.ok(tokens.includes('token'), 'should include token');
  assert.ok(tokens.includes('change'), 'should include change');
  assert.ok(!tokens.includes('the'), 'should drop short token');
  assert.ok(!tokens.includes('reproduce'), 'should drop stop word');
});

test('extractRepoTokens — falls back to whole body when no header', () => {
  const body = 'some random rotation issue affecting addresses';
  const tokens = extractRepoTokens(body);
  assert.ok(tokens.includes('rotation'));
  assert.ok(tokens.includes('addresses'));
});

test('extractRepoTokens — empty body returns empty', () => {
  assert.deepStrictEqual(extractRepoTokens(''), []);
  assert.deepStrictEqual(extractRepoTokens(null), []);
});

test('extractRepoTokens — dedupes', () => {
  const tokens = extractRepoTokens('rotation rotation rotation');
  assert.deepStrictEqual(tokens, ['rotation']);
});

// ---------------------------------------------------------------------------
// labelNamesOf + isBug
// ---------------------------------------------------------------------------

test('labelNamesOf — handles object form', () => {
  const issue = { labels: [{ name: 'bug' }, { name: 'urgent' }] };
  assert.deepStrictEqual(labelNamesOf(issue), ['bug', 'urgent']);
});

test('labelNamesOf — handles string form', () => {
  const issue = { labels: ['bug', 'urgent'] };
  assert.deepStrictEqual(labelNamesOf(issue), ['bug', 'urgent']);
});

test('labelNamesOf — handles missing labels', () => {
  assert.deepStrictEqual(labelNamesOf({}), []);
});

test('isBug — matches type: bug', () => {
  assert.strictEqual(isBug(['type: bug']), true);
});

test('isBug — matches bare bug', () => {
  assert.strictEqual(isBug(['bug']), true);
});

test('isBug — does not match feature', () => {
  assert.strictEqual(isBug(['type: feature', 'fn:dev']), false);
});

// ---------------------------------------------------------------------------
// detectNewTestFuncs
// ---------------------------------------------------------------------------

test('detectNewTestFuncs — finds new test from patch', async () => {
  const github = makeGithub({
    prFiles: [{
      filename: 'pkg/foo/foo_test.go',
      status: 'modified',
      patch: '@@ -1,3 +1,8 @@\n+func TestFooBar(t *testing.T) {\n+  // body\n+}\n',
    }],
    contentByRefPath: {
      'h:pkg/foo/foo_test.go': 'package foo\nfunc TestFooBar(t *testing.T) {}\nfunc TestExisting(t *testing.T) {}\n',
      'b:pkg/foo/foo_test.go': 'package foo\nfunc TestExisting(t *testing.T) {}\n'
    }
  });
  const core = mockCore();
  const ctx = makeContext({ pr: { number: 5, head: { sha: 'h' }, base: { sha: 'b' } } });
  const result = await detectNewTestFuncs({ github, context: ctx, core, pr: ctx.payload.pull_request });
  assert.ok(result.newTestFuncs.includes('TestFooBar'), 'should find new func');
  assert.ok(!result.newTestFuncs.includes('TestExisting'), 'should not flag preexisting func as new');
});

test('detectNewTestFuncs — added file: all funcs are new', async () => {
  const github = makeGithub({
    prFiles: [{
      filename: 'pkg/new_test.go',
      status: 'added',
      patch: '@@ +1,5 @@\n+func TestNewFunc(t *testing.T) {}\n'
    }],
    contentByRefPath: {
      'h:pkg/new_test.go': 'package pkg\nfunc TestNewFunc(t *testing.T) {}\n'
    }
  });
  const core = mockCore();
  const ctx = makeContext();
  const result = await detectNewTestFuncs({ github, context: ctx, core, pr: ctx.payload.pull_request });
  assert.deepStrictEqual(result.newTestFuncs, ['TestNewFunc']);
});

test('detectNewTestFuncs — removed file is skipped', async () => {
  const github = makeGithub({
    prFiles: [{
      filename: 'pkg/gone_test.go',
      status: 'removed',
      patch: '@@ -1,5 +0,0 @@\n-func TestGone(t *testing.T) {}\n'
    }]
  });
  const core = mockCore();
  const ctx = makeContext();
  const result = await detectNewTestFuncs({ github, context: ctx, core, pr: ctx.payload.pull_request });
  assert.deepStrictEqual(result.newTestFuncs, []);
});

test('detectNewTestFuncs — renamed file uses previous_filename for base', async () => {
  const github = makeGithub({
    prFiles: [{
      filename: 'pkg/new_name_test.go',
      previous_filename: 'pkg/old_name_test.go',
      status: 'renamed',
      patch: null  // patch often missing on rename-only
    }],
    contentByRefPath: {
      'h:pkg/new_name_test.go': 'package pkg\nfunc TestOriginal(t *testing.T) {}\nfunc TestNewlyAdded(t *testing.T) {}\n',
      'b:pkg/old_name_test.go': 'package pkg\nfunc TestOriginal(t *testing.T) {}\n'
    }
  });
  const core = mockCore();
  const ctx = makeContext();
  const result = await detectNewTestFuncs({ github, context: ctx, core, pr: ctx.payload.pull_request });
  assert.ok(result.newTestFuncs.includes('TestNewlyAdded'));
  assert.ok(!result.newTestFuncs.includes('TestOriginal'));
});

test('detectNewTestFuncs — file with no available content is indeterminate', async () => {
  const github = makeGithub({
    prFiles: [{
      filename: 'pkg/big_test.go',
      status: 'modified',
      patch: null
    }],
    contentByRefPath: {} // getContent will 404
  });
  const core = mockCore();
  const ctx = makeContext();
  const result = await detectNewTestFuncs({ github, context: ctx, core, pr: ctx.payload.pull_request });
  assert.deepStrictEqual(result.newTestFuncs, []);
  assert.deepStrictEqual(result.indeterminateFiles, ['pkg/big_test.go']);
});

test('detectNewTestFuncs — non-test file is ignored', async () => {
  const github = makeGithub({
    prFiles: [{
      filename: 'pkg/foo.go',
      status: 'modified',
      patch: '+func TestNotReally(t *testing.T) {}\n'
    }]
  });
  const core = mockCore();
  const ctx = makeContext();
  const result = await detectNewTestFuncs({ github, context: ctx, core, pr: ctx.payload.pull_request });
  assert.deepStrictEqual(result.newTestFuncs, []);
});

// ---------------------------------------------------------------------------
// handler — non-bug auto-close path
// ---------------------------------------------------------------------------

test('handler — non-bug issue auto-closes', async () => {
  const recordCalls = [];
  const github = makeGithub({
    issuesData: {
      42: { number: 42, body: '', user: { login: 'someone' }, labels: [{ name: 'type: feature' }] }
    },
    recordCalls
  });
  const core = mockCore();
  const ctx = makeContext({
    pr: { number: 99, title: 'impl: Issue #42 - feature work', body: '' }
  });

  await handler({ github, context: ctx, core });

  const update = recordCalls.find(c => c.kind === 'update');
  const comment = recordCalls.find(c => c.kind === 'createComment');
  assert.ok(update, 'should close the issue');
  assert.strictEqual(update.params.state, 'closed');
  assert.ok(comment.params.body.includes('Resolved by PR #99'));
});

// ---------------------------------------------------------------------------
// handler — bug, gates pass
// ---------------------------------------------------------------------------

test('handler — bug with new test + repro keyword: awaiting-verification', async () => {
  const recordCalls = [];
  const github = makeGithub({
    issuesData: {
      540: {
        number: 540,
        user: { login: 'reporter' },
        labels: [{ name: 'type: bug' }, { name: 'copilot-triaging' }],
        body: `### Steps to Reproduce\n\nwgmesh rotation breaks ip\n`
      }
    },
    prFiles: [{
      filename: 'pkg/daemon/rotation_test.go',
      status: 'added',
      patch: '@@ +1,5 @@\n+func TestRotationKeepsMeshIP(t *testing.T) {}\n'
    }],
    contentByRefPath: {
      'h:pkg/daemon/rotation_test.go': 'package daemon\nfunc TestRotationKeepsMeshIP(t *testing.T) {}\n'
    },
    recordCalls
  });
  const core = mockCore();
  const ctx = makeContext({
    pr: {
      number: 700,
      title: 'impl: Issue #540 - persist mesh ip across rotation',
      body: 'Adds TestRotationKeepsMeshIP exercising the rotation reproduction.'
    }
  });

  await handler({ github, context: ctx, core });

  const adds = recordCalls.filter(c => c.kind === 'addLabels');
  const removes = recordCalls.filter(c => c.kind === 'removeLabel');
  const comments = recordCalls.filter(c => c.kind === 'createComment');

  assert.ok(adds.some(a => a.params.labels.includes('awaiting-verification')), 'should add awaiting-verification');
  assert.ok(removes.some(r => r.params.name === 'copilot-triaging'), 'should remove copilot-triaging');
  assert.ok(comments[0].params.body.includes('Test gate passed'));
  assert.ok(comments[0].params.body.includes('TestRotationKeepsMeshIP'));
});

// ---------------------------------------------------------------------------
// handler — bug, L2 fails (no new test)
// ---------------------------------------------------------------------------

test('handler — bug without new test: awaiting-tests, no auto-close', async () => {
  const recordCalls = [];
  const github = makeGithub({
    issuesData: {
      540: {
        number: 540,
        user: { login: 'reporter' },
        labels: [{ name: 'type: bug' }, { name: 'awaiting-verification' }],
        body: `### Steps to Reproduce\n\nrotate the secret\n`
      }
    },
    prFiles: [{
      filename: 'pkg/daemon/daemon.go',
      status: 'modified',
      patch: '+// fix code\n'
    }],
    recordCalls
  });
  const core = mockCore();
  const ctx = makeContext({
    pr: { number: 701, title: 'impl: Issue #540 - fix without test', body: 'Fixes the rotation bug.' }
  });

  await handler({ github, context: ctx, core });

  const adds = recordCalls.filter(c => c.kind === 'addLabels');
  const removes = recordCalls.filter(c => c.kind === 'removeLabel');
  const updates = recordCalls.filter(c => c.kind === 'update');
  const comments = recordCalls.filter(c => c.kind === 'createComment');

  assert.ok(adds.some(a => a.params.labels.includes('awaiting-tests')), 'should add awaiting-tests');
  assert.ok(removes.some(r => r.params.name === 'awaiting-verification'),
    'should remove stale awaiting-verification to close the bypass race window');
  assert.strictEqual(updates.length, 0, 'should NOT close the issue');
  assert.ok(comments[0].params.body.includes('does not yet meet'));
  assert.ok(comments[0].params.body.includes('L2'));
});

// ---------------------------------------------------------------------------
// handler — bug, L3 fails (no keyword match)
// ---------------------------------------------------------------------------

test('handler — bug with new test but no keyword match: awaiting-tests', async () => {
  const recordCalls = [];
  const github = makeGithub({
    issuesData: {
      540: {
        number: 540,
        user: { login: 'reporter' },
        labels: [{ name: 'type: bug' }],
        body: `### Steps to Reproduce\n\nwireguard mesh rotation breaks identity\n`
      }
    },
    prFiles: [{
      filename: 'pkg/unrelated_test.go',
      status: 'added',
      patch: '+func TestNothingMatches(t *testing.T) {}\n'
    }],
    contentByRefPath: {
      'h:pkg/unrelated_test.go': 'package unrelated\nfunc TestNothingMatches(t *testing.T) {}\n'
    },
    recordCalls
  });
  const core = mockCore();
  const ctx = makeContext({
    pr: { number: 702, title: 'impl: Issue #540 - cosmetic test', body: 'Adds a test.' }
  });

  await handler({ github, context: ctx, core });

  const adds = recordCalls.filter(c => c.kind === 'addLabels');
  const updates = recordCalls.filter(c => c.kind === 'update');
  const comments = recordCalls.filter(c => c.kind === 'createComment');

  assert.ok(adds.some(a => a.params.labels.includes('awaiting-tests')), 'should add awaiting-tests on L3 fail');
  assert.strictEqual(updates.length, 0, 'should NOT close');
  assert.ok(comments[0].params.body.includes('L3'));
});

// ---------------------------------------------------------------------------
// handler — non-impl PR title is ignored
// ---------------------------------------------------------------------------

test('handler — PR title without Issue #N is no-op', async () => {
  const recordCalls = [];
  const github = makeGithub({ recordCalls });
  const core = mockCore();
  const ctx = makeContext({ pr: { number: 1, title: 'chore: cleanup' } });
  await handler({ github, context: ctx, core });
  assert.strictEqual(recordCalls.length, 0);
});

// ---------------------------------------------------------------------------
// handler — race-window order: remove before add
// ---------------------------------------------------------------------------

test('handler — failure path removes stale labels BEFORE adding awaiting-tests', async () => {
  const orderedCalls = [];
  const github = makeGithub({
    issuesData: {
      540: {
        number: 540,
        user: { login: 'reporter' },
        labels: [{ name: 'type: bug' }, { name: 'awaiting-verification' }],
        body: ''
      }
    },
    prFiles: [],
    recordCalls: orderedCalls
  });
  const core = mockCore();
  const ctx = makeContext({
    pr: { number: 705, title: 'impl: Issue #540 - empty', body: '' }
  });

  await handler({ github, context: ctx, core });

  const labelMutations = orderedCalls.filter(c => c.kind === 'removeLabel' || c.kind === 'addLabels');
  const removeIdx = labelMutations.findIndex(c => c.kind === 'removeLabel' && c.params.name === 'awaiting-verification');
  const addIdx = labelMutations.findIndex(c => c.kind === 'addLabels' && c.params.labels.includes('awaiting-tests'));
  assert.ok(removeIdx >= 0 && addIdx >= 0, 'both calls should occur');
  assert.ok(removeIdx < addIdx, `remove (idx ${removeIdx}) must precede add (idx ${addIdx}) to close race window`);
});

// ---------------------------------------------------------------------------
// handler — success path: ADD before REMOVE so verify-comment-close can fire
// ---------------------------------------------------------------------------

test('handler — success path adds awaiting-verification BEFORE removing stale labels', async () => {
  const orderedCalls = [];
  const github = makeGithub({
    issuesData: {
      540: {
        number: 540,
        user: { login: 'reporter' },
        labels: [{ name: 'type: bug' }, { name: 'awaiting-tests' }],
        body: '### Steps to Reproduce\n\nrotation\n'
      }
    },
    prFiles: [{
      filename: 'pkg/rotation_test.go',
      status: 'added',
      patch: '+func TestRotationFix(t *testing.T) {}\n'
    }],
    contentByRefPath: {
      'h:pkg/rotation_test.go': 'package x\nfunc TestRotationFix(t *testing.T) {}\n'
    },
    recordCalls: orderedCalls
  });
  const core = mockCore();
  const ctx = makeContext({
    pr: { number: 800, title: 'impl: Issue #540 - rotation fix', body: 'Adds TestRotationFix.' }
  });

  await handler({ github, context: ctx, core });

  const labelMutations = orderedCalls.filter(c => c.kind === 'removeLabel' || c.kind === 'addLabels');
  const addIdx = labelMutations.findIndex(c => c.kind === 'addLabels' && c.params.labels.includes('awaiting-verification'));
  const removeIdx = labelMutations.findIndex(c => c.kind === 'removeLabel' && c.params.name === 'awaiting-tests');
  assert.ok(addIdx >= 0 && removeIdx >= 0, 'both calls should occur on success path');
  assert.ok(addIdx < removeIdx, `add (idx ${addIdx}) must precede remove (idx ${removeIdx}) on success path so verify-comment-close.yml never sees a label-less window`);
});

// ---------------------------------------------------------------------------
// detectNewTestFuncs — empty file produces empty Set, not indeterminate
// ---------------------------------------------------------------------------

test('detectNewTestFuncs — empty test file is empty, not indeterminate', async () => {
  const github = makeGithub({
    prFiles: [{
      filename: 'pkg/empty_test.go',
      status: 'added',
      patch: null
    }],
    contentByRefPath: {
      'h:pkg/empty_test.go': ''
    }
  });
  const core = mockCore();
  const ctx = makeContext();
  const result = await detectNewTestFuncs({ github, context: ctx, core, pr: ctx.payload.pull_request });
  assert.deepStrictEqual(result.newTestFuncs, []);
  assert.deepStrictEqual(result.indeterminateFiles, []);
});

// ---------------------------------------------------------------------------
// detectNewTestFuncs — content fallback is conditional (skips when patch found tests)
// ---------------------------------------------------------------------------

test('detectNewTestFuncs — content fallback runs even when patch parsing succeeds', async () => {
  // Always-on fallback: GitHub can truncate large diffs mid-file with
  // some `+func Test...` matches showing but others below the cutoff.
  // Conditional skip risks L3 false negatives. Always run; cost is 1-2
  // extra getContent calls per *_test.go file.
  let getContentCalls = 0;
  const github = {
    paginate: async () => [{
      filename: 'pkg/foo_test.go',
      status: 'modified',
      patch: '+func TestFromPatch(t *testing.T) {}\n'
    }],
    rest: {
      pulls: { listFiles: Object.assign(async () => {}, { _kind: 'listFiles' }) },
      repos: {
        getContent: async ({ ref }) => {
          getContentCalls++;
          // Return both head and base content so the diff yields TestFromPatch as new.
          if (ref === 'h') {
            return { data: { content: Buffer.from('package foo\nfunc TestFromPatch(t *testing.T) {}\n', 'utf-8').toString('base64'), encoding: 'base64' } };
          }
          return { data: { content: Buffer.from('package foo\n', 'utf-8').toString('base64'), encoding: 'base64' } };
        }
      }
    }
  };
  const core = mockCore();
  const ctx = makeContext();
  const result = await detectNewTestFuncs({ github, context: ctx, core, pr: ctx.payload.pull_request });
  assert.deepStrictEqual(result.newTestFuncs, ['TestFromPatch']);
  assert.ok(getContentCalls >= 1, 'getContent MUST be called even when patch produced matches (truncation defense)');
});

// ---------------------------------------------------------------------------
// handler — reopen-on-bypass: GitHub native Closes #N closed the issue
// before the workflow ran. Handler must detect + reopen + run gate.
// ---------------------------------------------------------------------------

test('handler — bug already closed (no gate labels) gets reopened before gate runs', async () => {
  const recordCalls = [];
  const github = makeGithub({
    issuesData: {
      540: {
        number: 540,
        state: 'closed',
        user: { login: 'reporter' },
        labels: [{ name: 'type: bug' }, { name: 'copilot-triaging' }],
        body: '### Steps to Reproduce\n\nrotation breaks ip\n'
      }
    },
    prFiles: [{
      filename: 'pkg/rotation_test.go',
      status: 'added',
      patch: '+func TestRotationFix(t *testing.T) {}\n'
    }],
    contentByRefPath: {
      'h:pkg/rotation_test.go': 'package x\nfunc TestRotationFix(t *testing.T) {}\n'
    },
    recordCalls
  });
  const core = mockCore();
  const ctx = makeContext({
    pr: { number: 800, title: 'impl: Issue #540 - rotation fix', body: 'Closes #540. Adds TestRotationFix.' }
  });

  await handler({ github, context: ctx, core });

  const reopens = recordCalls.filter(c => c.kind === 'update' && c.params.state === 'open');
  assert.strictEqual(reopens.length, 1, 'must reopen the bypassed issue');
  assert.strictEqual(reopens[0].params.state, 'open');
  // state_reason intentionally omitted — GitHub API can 422 on `state_reason: 'reopened'`
  // for re-open transitions. State change alone is sufficient.

  const adds = recordCalls.filter(c => c.kind === 'addLabels');
  assert.ok(adds.some(a => a.params.labels.includes('awaiting-verification')),
    'gate should run + add awaiting-verification after reopen');

  const comments = recordCalls.filter(c => c.kind === 'createComment');
  assert.ok(comments[0].params.body.includes('auto-closed'),
    'success comment should mention the bypass auto-close');
});

test('handler — bug closed WITH awaiting-verification label is NOT reopened', async () => {
  const recordCalls = [];
  const github = makeGithub({
    issuesData: {
      540: {
        number: 540,
        state: 'closed',
        user: { login: 'reporter' },
        labels: [{ name: 'type: bug' }, { name: 'awaiting-verification' }],
        body: ''
      }
    },
    prFiles: [],
    recordCalls
  });
  const core = mockCore();
  const ctx = makeContext({
    pr: { number: 801, title: 'impl: Issue #540 - cleanup', body: '' }
  });

  await handler({ github, context: ctx, core });

  const reopens = recordCalls.filter(c => c.kind === 'update' && c.params.state === 'open');
  assert.strictEqual(reopens.length, 0, 'must NOT reopen if awaiting-verification present (legitimate close path)');
});

// ---------------------------------------------------------------------------
// detectNewTestFuncs — indeterminate ONLY when both patch and content fail
// ---------------------------------------------------------------------------

test('detectNewTestFuncs — patch with matches + getContent fails: NOT indeterminate', async () => {
  const github = makeGithub({
    prFiles: [{
      filename: 'pkg/foo_test.go',
      status: 'modified',
      patch: '+func TestFromPatch(t *testing.T) {}\n'
    }],
    contentByRefPath: {} // getContent will 404
  });
  const core = mockCore();
  const ctx = makeContext();
  const result = await detectNewTestFuncs({ github, context: ctx, core, pr: ctx.payload.pull_request });
  // Patch found a test, so file is NOT indeterminate even though content failed
  assert.deepStrictEqual(result.newTestFuncs, ['TestFromPatch']);
  assert.deepStrictEqual(result.indeterminateFiles, []);
});
