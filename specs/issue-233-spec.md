# Specification: Issue #233

## Classification
fix

## Deliverables
code

## Problem Analysis

The dashboard (`docs/index.html`) has a dedicated **mem0 Agent Memory** section that is expected to show:
- Current mem0 health status (Healthy / Degraded / Disabled)
- Retrieve step result from the latest Goose Implementation run
- Save step result from the latest Goose Implementation run
- Cache status (Encrypted + Cached / Cached / Not cached)
- A history list of the last 5 runs

The `fetchMem0Stats` function in `docs/index.html` retrieves this data by:
1. Filtering `workflow_runs` (already fetched) for completed "Goose Implementation" runs
2. Calling the Jobs API (`/actions/runs/{id}/jobs`) for each run to inspect step conclusions

Two bugs prevent the panel from populating reliably:

**Bug 1 – `per_page=40` often excludes Goose runs**

The main run list is fetched with `/actions/runs?per_page=40`. The repository runs many other workflows (triage, spec-approve, board-sync, metrics-report, etc.) on every push and issue event. When those workflows dominate the last 40 entries, no "Goose Implementation" run appears in the list and `gooseRuns.length === 0` triggers `renderMem0Empty()`, showing dashes for all fields.

**Bug 2 – JavaScript operator-precedence bug in the job-finding predicate**

```js
j.steps.some(s =>
  s.name &&
  (s.name.includes('Retrieve memories' ||   // <── evaluated first by JS
   'Save memories' ||                         //     returns 'Retrieve memories'
   'Encrypt mem0' ||
   'Save mem0 memory cache' ||
   'Restore mem0 memory cache'))
)
```

Because `||` is evaluated *inside* the `includes()` argument list, the entire chain short-circuits to `'Retrieve memories'`. The predicate therefore only matches jobs that contain a step named exactly matching 'Retrieve memories…'. Although this is partially mitigated by the `|| jobs[0]` fallback, it is semantically wrong and will break silently if job ordering changes.

**Secondary issue – `fetched >= 2` cap**

The per-cycle fetch limit of 2 new job API calls means only 2 runs are processed per full cycle (~15 min). On cold start this is acceptable, but combined with Bug 1 it means that if the two fetched runs both belong to the `mem0Cache` already, no new data is rendered even when fresh runs exist.

## Proposed Approach

### Fix 1 – Fetch Goose-specific runs via workflow file name filter

Replace:
```js
ghFetch('/actions/runs?per_page=40')
```
with a second dedicated fetch for Goose runs that uses the `workflow_file_name` query parameter GitHub supports:
```js
ghFetch('/actions/runs?workflow_id=goose-build.yml&per_page=10&status=completed')
```

This guarantees that the mem0 panel always has fresh Goose run data regardless of how many other workflows ran recently. The result should be stored separately from the general `runs` variable and passed directly to `fetchMem0Stats`.

### Fix 2 – Correct the operator-precedence bug

Replace the single `includes(A || B || C)` call with properly separated conditions:

```js
j.steps.some(s =>
  s.name && (
    s.name.includes('Retrieve memories') ||
    s.name.includes('Save memories') ||
    s.name.includes('Encrypt mem0') ||
    s.name.includes('Save mem0 memory cache') ||
    s.name.includes('Restore mem0 memory cache')
  )
)
```

### Fix 3 (optional) – Raise or remove the `fetched >= 2` cap

Since the dedicated Goose-run fetch will return at most 10 entries (typically 1–3 recent runs), the cap of 2 can be raised to 5 to match the `.slice(0, 5)` already in place:

```js
if (mem0Cache[run.id] || fetched >= 5) continue;
```

## Affected Files

| File | Change |
|------|--------|
| `docs/index.html` | Fix Bug 1: add dedicated Goose workflow run fetch in `fetchAll`. Fix Bug 2: correct `includes()` operator precedence. Optionally raise `fetched` cap to 5. |

## Test Strategy

1. **Manual verification**: Open `docs/index.html` in a browser (pointing to the chimney proxy or directly to `api.github.com`). Confirm the mem0 section displays a status badge, retrieve/save indicators, and a populated run history list instead of dashes.
2. **Regression check**: Verify that the DORA, Pipeline Tracker, and Active Runs panels still populate correctly (the general `runs` fetch is unchanged).
3. **Rate limit check**: Confirm that the new dedicated fetch counts against the same `ghFetch` rate-limit guard and that the dashboard does not exceed the 60 req/hour unauthenticated limit during a normal session (new fetch adds at most 1 additional call per full cycle).

## Estimated Complexity
low
