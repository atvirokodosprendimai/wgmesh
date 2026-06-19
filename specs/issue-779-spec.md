# Issue #779: Implement Trial Signup Analytics Funnel

## Classification
feature

## Problem Analysis

The wgmesh trial signup flow currently lacks visibility into user behavior patterns. Without granular analytics tracking at each step of the signup process, product and engineering teams cannot:

1. Identify where users abandon the signup flow
2. Measure conversion rates between signup stages
3. Detect technical issues preventing successful signups
4. Optimize the user experience based on data
5. Attribute trial activations to marketing channels

Current state:
- Basic "trial started" event exists
- No step-by-step progression tracking
- No error context captured when failures occur
- No user session correlation for dropped-off users

Business impact:
- Unknown conversion rate from landing page → active trial
- Unable to prioritize UX improvements quantitatively
- No visibility into technical friction points
- Limited ability to measure A/B test impact on signup flow

## Proposed Approach

### 1. Define Signup Funnel Stages

Track the following sequential stages:

| Stage | Event Name | Description |
|-------|------------|-------------|
| 1 | `trial_landing_viewed` | User lands on trial signup page |
| 2 | `trial_form_started` | User begins entering email/details |
| 3 | `trial_email_submitted` | Email submitted, awaiting verification |
| 4 | `trial_email_verified` | User clicked verification link |
| 5 | `trial_account_created` | Backend account creation successful |
| 6 | `trial_install_started` | User begins wgmesh install |
| 7 | `trial_install_completed` | wgmesh install finished |
| 8 | `trial_mesh_active` | First mesh network operational |

### 2. Instrumentation Architecture

**Frontend Tracking (Signup UI):**
```javascript
// Event schema
{
  event_id: string (UUID),
  event_type: string (enum from funnel stages),
  timestamp: ISO8601,
  session_id: string (persistent across funnel),
  user_id: string|null (anonymous until verified),
  metadata: {
    referrer: string,
    utm_source: string|null,
    utm_medium: string|null,
    utm_campaign: string|null,
    browser: string,
    device_type: string
  }
}
```

**Backend Tracking (API Events):**
- Emit events when verification emails sent
- Track email link clicks via verification token metadata
- Emit events on account creation success/failure
- Track mesh activation events from daemon

### 3. Error and Drop-off Tracking

For each stage that can fail, capture:

```typescript
interface FunnelError {
  stage: string;
  error_type: string;  // e.g., "validation_error", "rate_limit", "server_error"
  error_code: string;
  user_message: string|null;  // What the user saw
  context: Record<string, unknown>;
}
```

### 4. Storage and Aggregation

**Events Collection:**
- Use existing analytics pipeline (PostgreSQL + TimescaleDB for events)
- Create `trial_signup_events` table with indexes on `session_id` and `user_id`
- Retention policy: 90 days raw events, aggregated funnel metrics indefinitely

**Funnel Query:**
```sql
-- Daily funnel conversion rates
WITH funnel_stages AS (
  SELECT
    date_trunc('day', timestamp) as date,
    event_type,
    COUNT(DISTINCT session_id) as unique_sessions
  FROM trial_signup_events
  WHERE timestamp >= NOW() - INTERVAL '30 days'
  GROUP BY 1, 2
)
SELECT
  date,
  event_type,
  unique_sessions,
  LAG(unique_sessions) OVER (PARTITION BY date ORDER BY event_type) as previous_stage,
  (unique_sessions::float / NULLIF(LAG(unique_sessions) OVER (PARTITION BY date ORDER BY event_type), 0)) * 100 as conversion_rate_pct
FROM funnel_stages
ORDER BY date, event_type;
```

### 5. Dashboard Implementation

Create admin dashboard showing:
- Daily/weekly funnel visualization
- Stage-by-stage conversion rates
- Drop-off volume by stage
- Error breakdown by stage
- Conversion by referral source

### 6. Implementation Phases

**Phase 1 - Frontend Instrumentation:**
- Add analytics SDK to signup flow pages
- Instrument form interactions and page views
- Generate and persist session IDs

**Phase 2 - Backend Events:**
- Add event emission to verification endpoints
- Track account creation outcomes
- Correlate verification tokens with sessions

**Phase 3 - Mesh Activation Tracking:**
- Emit trial activation events from mesh daemon
- Link daemon events to user accounts

**Phase 4 - Dashboard and Queries:**
- Build funnel aggregation queries
- Create admin dashboard views
- Set up alerting for anomalous drop-off rates

## Acceptance Criteria

### Tracking Verification
- [ ] All 8 funnel stages emit events to analytics pipeline
- [ ] Each event includes required metadata (session_id, timestamp, user_id when available)
- [ ] Session IDs persist across all user interactions in signup flow
- [ ] Events are emitted within 500ms of user action

### Data Completeness
- [ ] At least 95% of signup sessions have complete event chains (from landing to current stage)
- [ ] Error events capture error type, code, and user-facing message
- [ ] UTM parameters captured from initial landing page

### Query Capability
- [ ] Funnel aggregation query completes in <5 seconds for 90-day window
- [ ] Dashboard displays real-time funnel data (within 5 minutes)
- [ ] Can filter funnel by date range, referral source, device type

### Validation
- [ ] Manual test signup produces complete event chain
- [ ] Intentionally failing each stage produces appropriate error event
- [ ] Session correlation works across email verification boundary (anonymous → authenticated)

### Monitoring
- [ ] Alert configured for drop-off rate >50% at any stage (24hr moving average)
- [ ] Alert configured for error rate >5% at any stage
- [ ] Dashboard accessible to product and engineering roles

## Out of scope

- **Marketing attribution beyond UTM tags:** No integration with external marketing platforms (Google Ads, Meta, etc.)
- **User behavior within mesh:** Tracking only covers signup through activation; no post-activation usage analytics
- **A/B testing infrastructure:** Funnel measurement supports A/B tests but building the A/B framework is separate work
- **Real-time alerting on individual sessions:** Alerts are aggregate-based; no per-user signup failure notifications
- **Historical data migration:** Only new signups after implementation will have complete funnel data
- **PII beyond email:** No collection of names, phone numbers, or other personal identifiers beyond email (already required for signup)
- **Cross-domain tracking:** Assumes signup flow occurs on same domain; no cross-site visitor correlation
- **Mobile app funnel:** Scope limited to web-based signup flow only
- **API-only signups:** Programmatic account creation via API not covered (can be added separately)
