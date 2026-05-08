// scripts/workflows/e2e-verify-close-handler.test.js
//
// Unit tests for e2e-verify-close-handler.js.
// Runtime: node --test scripts/workflows/e2e-verify-close-handler.test.js

'use strict';

const test = require('node:test');
const assert = require('node:assert');

const handler = require('./e2e-verify-close-handler.js');
const { extractIssueNumber, resolvePullRequest } = handler;

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

function mockCore() {
  return {
    info: () => {},
    warning: () => {}
  };
}

function makeGithub({
  issuesData = {},
  pullsData = {},
  commitMessageBySha = {},
  commentsData = {},
  listCommentsImpl,
  recordCalls = []
} = {}) {
  return {
    rest: {
      issues: {
        get: async ({ issue_number }) => {
          const issue = issuesData[issue_number];
          if (!issue) {
            const e = new Error(`issue ${issue_number} not found`);
            e.status = 404;
            throw e;
          }
          return { data: issue };
        },
        update: async (params) => { recordCalls.push({ kind: 'update', params }); },
        addLabels: async (params) => { recordCalls.push({ kind: 'addLabels', params }); },
        removeLabel: async (params) => {
          recordCalls.push({ kind: 'removeLabel', params });
          // Default: succeed. Tests that need 404 wire that into a custom mock.
        },
        listComments: listCommentsImpl || (async (params) => {
          recordCalls.push({ kind: 'listComments', params });
          return { data: commentsData[params.issue_number] || [] };
        }),
        createComment: async (params) => { recordCalls.push({ kind: 'createComment', params }); }
      },
      pulls: {
        get: async ({ pull_number }) => {
          const pr = pullsData[pull_number];
          if (!pr) {
            const e = new Error(`pull ${pull_number} not found`);
            e.status = 404;
            throw e;
          }
          return { data: pr };
        }
      },
      repos: {
        getCommit: async ({ ref }) => {
          const message = commitMessageBySha[ref];
          if (message === undefined) {
            const e = new Error(`commit ${ref} not found`);
            e.status = 404;
            throw e;
          }
          return { data: { commit: { message } } };
        }
      }
    }
  };
}

function makeContext({ owner = 'O', repo = 'R', workflowRun = {} } = {}) {
  return {
    repo: { owner, repo },
    payload: {
      workflow_run: {
        conclusion: 'success',
        head_sha: 'abc1234',
        html_url: 'https://example.com/run/1',
        pull_requests: [],
        ...workflowRun
      }
    }
  };
}

// ---------------------------------------------------------------------------
// extractIssueNumber
// ---------------------------------------------------------------------------

test('extractIssueNumber — pulls Issue #N out of impl PR title', () => {
  assert.strictEqual(extractIssueNumber('impl: Issue #556 - relay flap fix'), 556);
});

test('extractIssueNumber — returns null when no Issue #N reference', () => {
  assert.strictEqual(extractIssueNumber('chore: cleanup'), null);
});

test('extractIssueNumber — handles empty / null inputs', () => {
  assert.strictEqual(extractIssueNumber(''), null);
  assert.strictEqual(extractIssueNumber(null), null);
});

// ---------------------------------------------------------------------------
// handler — success path: addLabels(verified), remove in-flight labels,
// close, comment with run URL.
// ---------------------------------------------------------------------------

test('handler — workflow_run success closes issue, adds verified, removes awaiting-verification', async () => {
  const recordCalls = [];
  const github = makeGithub({
    issuesData: {
      556: { number: 556, state: 'open', labels: [{ name: 'awaiting-verification' }, { name: 'type: bug' }] }
    },
    pullsData: {
      564: { number: 564, title: 'impl: Issue #556 - relay flap fix' }
    },
    recordCalls
  });
  const ctx = makeContext({
    workflowRun: {
      conclusion: 'success',
      head_sha: 'sha564',
      html_url: 'https://github.com/o/r/actions/runs/100',
      pull_requests: [{ number: 564 }]
    }
  });

  await handler({ github, context: ctx, core: mockCore() });

  const adds = recordCalls.filter(c => c.kind === 'addLabels');
  const removes = recordCalls.filter(c => c.kind === 'removeLabel');
  const updates = recordCalls.filter(c => c.kind === 'update');
  const comments = recordCalls.filter(c => c.kind === 'createComment');

  assert.ok(adds.some(a => a.params.labels.includes('verified')), 'must add verified');
  assert.ok(removes.some(r => r.params.name === 'awaiting-verification'), 'must remove awaiting-verification');
  assert.ok(removes.some(r => r.params.name === 'awaiting-tests'), 'must remove awaiting-tests defensively');
  assert.ok(removes.some(r => r.params.name === 'e2e-failed'), 'must clear prior e2e-failed');
  assert.ok(removes.some(r => r.params.name === 'e2e-stalled'), 'must clear prior e2e-stalled');
  assert.strictEqual(updates.length, 1, 'must close the issue');
  assert.strictEqual(updates[0].params.state, 'closed');
  assert.ok(comments[0].params.body.includes('https://github.com/o/r/actions/runs/100'),
    'comment must include run URL');
});

test('handler — workflow_run success first run posts verifier comment', async () => {
  const recordCalls = [];
  const github = makeGithub({
    issuesData: {
      556: { number: 556, state: 'open', labels: [{ name: 'awaiting-verification' }] }
    },
    pullsData: {
      564: { number: 564, title: 'impl: Issue #556 - relay flap fix' }
    },
    commentsData: { 556: [] },
    recordCalls
  });
  const ctx = makeContext({
    workflowRun: {
      conclusion: 'success',
      html_url: 'https://github.com/o/r/actions/runs/101',
      pull_requests: [{ number: 564 }]
    }
  });

  await handler({ github, context: ctx, core: mockCore() });

  const listComments = recordCalls.filter(c => c.kind === 'listComments');
  const comments = recordCalls.filter(c => c.kind === 'createComment');
  assert.strictEqual(listComments.length, 1, 'must check recent comments before posting');
  assert.strictEqual(comments.length, 1, 'first run should post one verifier comment');
  assert.ok(comments[0].params.body.includes('https://github.com/o/r/actions/runs/101'));
});

test('handler — workflow_run replay with existing run URL skips duplicate comment', async () => {
  const recordCalls = [];
  const runUrl = 'https://github.com/o/r/actions/runs/102';
  const github = makeGithub({
    issuesData: {
      556: { number: 556, state: 'closed', labels: [{ name: 'verified' }] }
    },
    pullsData: {
      564: { number: 564, title: 'impl: Issue #556 - relay flap fix' }
    },
    commentsData: {
      556: [{ body: `Already handled by verifier run ${runUrl}` }]
    },
    recordCalls
  });
  const ctx = makeContext({
    workflowRun: {
      conclusion: 'success',
      html_url: runUrl,
      pull_requests: [{ number: 564 }]
    }
  });

  await handler({ github, context: ctx, core: mockCore() });

  assert.strictEqual(recordCalls.filter(c => c.kind === 'createComment').length, 0,
    'replayed workflow_run should not post a second comment for the same run URL');
});

test('handler — listComments failure falls through and posts verifier comment', async () => {
  const recordCalls = [];
  const github = makeGithub({
    issuesData: {
      556: { number: 556, state: 'open', labels: [{ name: 'awaiting-verification' }] }
    },
    pullsData: {
      564: { number: 564, title: 'impl: Issue #556 - relay flap fix' }
    },
    listCommentsImpl: async () => {
      throw new Error('comments unavailable');
    },
    recordCalls
  });
  const ctx = makeContext({
    workflowRun: {
      conclusion: 'failure',
      html_url: 'https://github.com/o/r/actions/runs/103',
      pull_requests: [{ number: 564 }]
    }
  });

  await handler({ github, context: ctx, core: mockCore() });

  assert.strictEqual(recordCalls.filter(c => c.kind === 'createComment').length, 1,
    'comment listing errors should not suppress verifier comments');
});

// ---------------------------------------------------------------------------
// handler — PR title without Issue #N is no-op.
// ---------------------------------------------------------------------------

test('handler — PR title without Issue #N: no API writes', async () => {
  const recordCalls = [];
  const github = makeGithub({
    pullsData: {
      900: { number: 900, title: 'chore: tidy go.mod' }
    },
    recordCalls
  });
  const ctx = makeContext({
    workflowRun: {
      conclusion: 'success',
      pull_requests: [{ number: 900 }]
    }
  });

  await handler({ github, context: ctx, core: mockCore() });

  assert.strictEqual(recordCalls.length, 0, 'no API writes when PR title lacks Issue #N');
});

// ---------------------------------------------------------------------------
// handler — already-closed + already-verified is idempotent.
// ---------------------------------------------------------------------------

test('handler — issue already closed + verified: idempotent (no second close, label re-add OK)', async () => {
  const recordCalls = [];
  const github = makeGithub({
    issuesData: {
      556: { number: 556, state: 'closed', labels: [{ name: 'verified' }] }
    },
    pullsData: {
      564: { number: 564, title: 'impl: Issue #556 - relay flap fix' }
    },
    recordCalls
  });
  const ctx = makeContext({
    workflowRun: {
      conclusion: 'success',
      pull_requests: [{ number: 564 }]
    }
  });

  await handler({ github, context: ctx, core: mockCore() });

  const updates = recordCalls.filter(c => c.kind === 'update');
  assert.strictEqual(updates.length, 0, 'must not re-close an already-closed issue');
  const adds = recordCalls.filter(c => c.kind === 'addLabels');
  assert.ok(adds.some(a => a.params.labels.includes('verified')),
    'addLabels remains idempotent on success path');
});

// ---------------------------------------------------------------------------
// handler — failure: addLabels(e2e-failed), remove awaiting-verification,
// reopen if currently closed, comment with artifact link.
// ---------------------------------------------------------------------------

test('handler — workflow_run failure on closed issue reopens + adds e2e-failed', async () => {
  const recordCalls = [];
  const github = makeGithub({
    issuesData: {
      556: { number: 556, state: 'closed', labels: [{ name: 'awaiting-verification' }] }
    },
    pullsData: {
      564: { number: 564, title: 'impl: Issue #556 - relay flap fix' }
    },
    recordCalls
  });
  const ctx = makeContext({
    workflowRun: {
      conclusion: 'failure',
      html_url: 'https://github.com/o/r/actions/runs/200',
      pull_requests: [{ number: 564 }]
    }
  });

  await handler({ github, context: ctx, core: mockCore() });

  const adds = recordCalls.filter(c => c.kind === 'addLabels');
  const removes = recordCalls.filter(c => c.kind === 'removeLabel');
  const updates = recordCalls.filter(c => c.kind === 'update');
  const comments = recordCalls.filter(c => c.kind === 'createComment');

  assert.ok(adds.some(a => a.params.labels.includes('e2e-failed')), 'must add e2e-failed');
  assert.ok(removes.some(r => r.params.name === 'awaiting-verification'), 'must remove awaiting-verification');
  assert.strictEqual(updates.length, 1, 'must reopen the closed issue');
  assert.strictEqual(updates[0].params.state, 'open');
  assert.ok(comments[0].params.body.includes('https://github.com/o/r/actions/runs/200'),
    'failure comment must include run URL');
  assert.ok(/artifact|trace\.jsonl|tier-summary/i.test(comments[0].params.body),
    'failure comment must surface artifact link path');
});

test('handler — workflow_run failure on already-open issue: no reopen, label flips only', async () => {
  const recordCalls = [];
  const github = makeGithub({
    issuesData: {
      556: { number: 556, state: 'open', labels: [{ name: 'awaiting-verification' }] }
    },
    pullsData: {
      564: { number: 564, title: 'impl: Issue #556 - relay flap fix' }
    },
    recordCalls
  });
  const ctx = makeContext({
    workflowRun: {
      conclusion: 'failure',
      pull_requests: [{ number: 564 }]
    }
  });

  await handler({ github, context: ctx, core: mockCore() });

  const updates = recordCalls.filter(c => c.kind === 'update');
  assert.strictEqual(updates.length, 0, 'must not reopen an already-open issue');
});

// ---------------------------------------------------------------------------
// handler — non-actionable conclusions exit cleanly.
// ---------------------------------------------------------------------------

test('handler — workflow_run cancelled is logged + exits without API writes', async () => {
  const recordCalls = [];
  const github = makeGithub({ recordCalls });
  const ctx = makeContext({
    workflowRun: {
      conclusion: 'cancelled',
      pull_requests: [{ number: 564 }]
    }
  });

  await handler({ github, context: ctx, core: mockCore() });

  assert.strictEqual(recordCalls.length, 0, 'no API writes for cancelled');
});

test('handler — workflow_run timed_out is logged + exits without API writes', async () => {
  const recordCalls = [];
  const github = makeGithub({ recordCalls });
  const ctx = makeContext({
    workflowRun: {
      conclusion: 'timed_out',
      pull_requests: [{ number: 564 }]
    }
  });

  await handler({ github, context: ctx, core: mockCore() });

  assert.strictEqual(recordCalls.length, 0, 'no API writes for timed_out');
});

// ---------------------------------------------------------------------------
// handler — empty pull_requests array falls back to commit message scan.
// ---------------------------------------------------------------------------

test('handler — empty pull_requests array uses commit message Issue #N fallback', async () => {
  const recordCalls = [];
  const github = makeGithub({
    issuesData: {
      556: { number: 556, state: 'open', labels: [] }
    },
    commitMessageBySha: {
      sha564: 'impl: Issue #556 - relay flap fix\n\nMerges PR …'
    },
    recordCalls
  });
  const ctx = makeContext({
    workflowRun: {
      conclusion: 'success',
      head_sha: 'sha564',
      html_url: 'https://example.com/run/1',
      pull_requests: []
    }
  });

  await handler({ github, context: ctx, core: mockCore() });

  const adds = recordCalls.filter(c => c.kind === 'addLabels');
  const updates = recordCalls.filter(c => c.kind === 'update');
  assert.ok(adds.some(a => a.params.labels.includes('verified')),
    'fallback path still adds verified');
  assert.strictEqual(updates.length, 1, 'fallback path still closes the issue');
  assert.strictEqual(updates[0].params.state, 'closed');
});

test('handler — empty pull_requests + commit message lacks Issue #N: clean exit', async () => {
  const recordCalls = [];
  const github = makeGithub({
    commitMessageBySha: {
      sha999: 'chore: bump deps'
    },
    recordCalls
  });
  const ctx = makeContext({
    workflowRun: {
      conclusion: 'success',
      head_sha: 'sha999',
      pull_requests: []
    }
  });

  await handler({ github, context: ctx, core: mockCore() });

  assert.strictEqual(recordCalls.length, 0, 'no writes when no Issue #N anywhere');
});

// ---------------------------------------------------------------------------
// handler — removeLabel 404 is tolerated (label not present).
// ---------------------------------------------------------------------------

test('handler — removeLabel 404 is swallowed (label not present)', async () => {
  // Custom mock: removeLabel always 404s. Handler must not throw.
  const recordCalls = [];
  const github = {
    rest: {
      issues: {
        get: async () => ({ data: { number: 556, state: 'open', labels: [] } }),
        update: async (params) => { recordCalls.push({ kind: 'update', params }); },
        addLabels: async (params) => { recordCalls.push({ kind: 'addLabels', params }); },
        removeLabel: async (params) => {
          recordCalls.push({ kind: 'removeLabel', params });
          const e = new Error('not found');
          e.status = 404;
          throw e;
        },
        createComment: async (params) => { recordCalls.push({ kind: 'createComment', params }); }
      },
      pulls: {
        get: async () => ({ data: { number: 564, title: 'impl: Issue #556 - fix' } })
      }
    }
  };
  const ctx = makeContext({
    workflowRun: {
      conclusion: 'success',
      pull_requests: [{ number: 564 }]
    }
  });

  // Must not throw.
  await handler({ github, context: ctx, core: mockCore() });
  assert.ok(recordCalls.some(c => c.kind === 'addLabels'), 'success path completes despite 404 removeLabel');
});

test('resolvePullRequest — merge commit body Issue #N fallback surfaces issue title', async () => {
  const github = makeGithub({
    commitMessageBySha: {
      sha555: 'Merge pull request #555 from feature/x\n\nThis change addresses Issue #123 by ...'
    }
  });
  const ctx = makeContext({
    workflowRun: {
      head_sha: 'sha555',
      pull_requests: []
    }
  });

  const resolved = await resolvePullRequest({
    github,
    context: ctx,
    core: mockCore(),
    workflowRun: ctx.payload.workflow_run
  });

  assert.deepStrictEqual(resolved, { prNumber: null, prTitle: 'Issue #123' });
  assert.strictEqual(extractIssueNumber(resolved.prTitle), 123);
});

test('handler — failure on closed issue with awaiting-verification reopens', async () => {
  const recordCalls = [];
  const github = makeGithub({
    issuesData: {
      556: { number: 556, state: 'closed', labels: ['awaiting-verification'] }
    },
    pullsData: {
      564: { number: 564, title: 'impl: Issue #556 - relay flap fix' }
    },
    recordCalls
  });
  const ctx = makeContext({
    workflowRun: {
      conclusion: 'failure',
      pull_requests: [{ number: 564 }]
    }
  });

  await handler({ github, context: ctx, core: mockCore() });

  assert.ok(recordCalls.some(c => c.kind === 'update' && c.params.state === 'open'),
    'closed verifier-controlled issue must be reopened');
});

test('handler — failure on reporter-closed issue without awaiting-verification skips reopen', async () => {
  const recordCalls = [];
  const infoCalls = [];
  const github = makeGithub({
    issuesData: {
      556: { number: 556, state: 'closed', labels: ['e2e-failed'] }
    },
    pullsData: {
      564: { number: 564, title: 'impl: Issue #556 - relay flap fix' }
    },
    recordCalls
  });
  const ctx = makeContext({
    workflowRun: {
      conclusion: 'failure',
      pull_requests: [{ number: 564 }]
    }
  });
  const core = {
    info: (msg) => { infoCalls.push(msg); },
    warning: () => {}
  };

  await handler({ github, context: ctx, core });

  assert.strictEqual(recordCalls.some(c => c.kind === 'update' && c.params.state === 'open'), false,
    'closed reporter-driven issue must not be reopened');
  assert.ok(infoCalls.some(msg => msg.includes('skipping reopen')),
    'must log why the closed issue was not reopened');
});
