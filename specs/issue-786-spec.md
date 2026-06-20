# Specification: Issue #786

## Classification
feature

## Problem Analysis

Cloudroof trial signups currently receive no automated follow-up. Users sign up for a trial but may not understand:
- How to properly configure and test their mesh network
- The range of use cases wgmesh enables (site-to-site, multi-cloud, remote teams, hybrid deployments)
- The value of upgrading to a paid tier (managed ingress, enterprise features, support)

This leads to low trial-to-paid conversion rates and users abandoning the trial before experiencing the full value of the product. A structured nurture sequence would:
- Guide users through successful onboarding and first mesh deployment
- Demonstrate real-world use cases relevant to their needs
- Build trust through educational content and success metrics
- Create timely conversion opportunities at key decision points

The nurture sequence needs to be automated, triggered by trial signup, and personalized where possible (e.g., referencing their trial tier, deployment progress). The emails should be concise, actionable, and avoid spam-like patterns.

## Deliverables
code

## Proposed Approach

### Phase 1: Email Sequence Content Strategy

Design a 5-email drip campaign with the following structure:

**Email 1: Welcome + Quick Start (Day 0)**
- Subject: "Welcome to Cloudroof - Let's get your mesh running"
- Content:
  - Trial activation confirmation and tier details
  - Link to quickstart guide (`docs/quickstart.md`)
  - First milestone: Get 3 nodes connected
  - Expected time: 15 minutes
  - Where to get help (support email, docs, troubleshooting guide)
- CTA: "Start Your First Mesh" (links to quickstart)

**Email 2: Milestone Check + Use Case Education (Day 2)**
- Subject: "How's your mesh coming along?"
- Content:
  - Check if they've completed first mesh (if tracking available)
  - 3 use case highlights with links to detailed docs:
    - Site-to-site VPN (`docs/use-cases/hybrid-site-to-site.md`)
    - Multi-cloud connectivity (`docs/use-cases/multi-cloud.md`)
    - Remote team access (`docs/use-cases/remote-dev-team.md`)
  - Success metric: "Teams with 5+ connected nodes see 90% fewer connectivity issues"
- CTA: "Explore Use Cases" (links to use cases hub)

**Email 3: Advanced Features + Managed Ingress Teaser (Day 5)**
- Subject: "Beyond basic meshing: Advanced Cloudroof features"
- Content:
  - Feature highlights: NAT traversal, relay fallback, DHT discovery
  - Teaser: Managed ingress (lighthouse integration)
  - Preview of paid tier benefits: CDN-backed ingress, automated xDS sync
  - How to test: Link to service CLI documentation
- CTA: "Try Managed Ingress" (links to service setup docs)

**Email 4: Trial Timeline + Upgrade Value (Day 12)**
- Subject: "Your trial is halfway done - Here's what you're missing"
- Content:
  - Trial expiration date and current usage stats (if available)
  - Free vs. Paid tier comparison matrix:
    - Free: Self-managed mesh, community support
    - Paid: Managed ingress, priority support, SLA
  - Success story/case study placeholder
  - Upgrade discount offer (optional: 20% off first month)
- CTA: "Compare Plans" or "Upgrade Now"

**Email 5: Final Push + Resources (Day 18)**
- Subject: "Last chance to upgrade your Cloudroof trial"
- Content:
  - Trial expiration reminder (X days remaining)
  - Resources for continued success on free tier:
    - FAQ link (`docs/FAQ.md`)
    - Troubleshooting guide (`docs/troubleshooting.md`)
    - Evaluation checklist (`docs/evaluation-checklist.md`)
  - Final upgrade CTA emphasizing risk of losing mesh continuity
  - Option to extend trial (contact support)
- CTAs: "Upgrade Now" + "Request Trial Extension"

### Phase 2: Technical Implementation

**Task 1: Create email templates structure**
- Create `cmd/chimney/email-templates/` directory
- Create 5 template files: `email-01-welcome.txt`, `email-02-usecases.txt`, `email-03-features.txt`, `email-04-comparison.txt`, `email-05-final.txt`
- Use Go template syntax for personalization: `{{.TrialID}}`, `{{.ExpiryDate}}`, `{{.UpgradeLink}}`
- Create text and HTML versions (start with text-only for MVP)

**Task 2: Add email service to chimney**
- In `cmd/chimney/`, add `email-service.go` with:
  - `EmailConfig` struct (SMTP settings, API keys)
  - `EmailSender` interface for pluggable providers
  - Template renderer using `text/template`
  - `SendWelcomeEmail`, `SendUseCaseEmail`, etc. methods
  - Retry logic and rate limiting

**Task 3: Implement scheduling and triggers**
- Add email schedule configuration to chimney config
- Integrate with trial signup webhook/handler to trigger Email 1
- Implement delayed send for Emails 2-5 using:
  - Database-backed job queue (preferred for reliability)
  - Or time-based scheduling with cron-like logic (simpler MVP)
- Track sent emails to avoid duplicates

**Task 4: Add unsubscribe and preference management**
- Add unsubscribe link to all emails (required by CAN-SPAM)
- Create `/unsubscribe` endpoint in chimney
- Store opt-out status in user profile

**Task 5: Analytics and tracking**
- Add open tracking (pixel or link wrapping)
- Add click tracking for CTAs
- Store engagement data for sequence optimization
- Metrics: open rate, click rate, conversion rate per email

### Phase 3: Testing and Validation

**Task 1: Write email content tests**
- Test template rendering with sample data
- Verify all links are valid
- Check unsubscribe links work
- Validate personalization fields

**Task 2: Integration tests**
- Test full sequence: signup → email 1 → email 2 → ... → email 5
- Test scheduling and timing accuracy
- Test error handling (SMTP failures, rate limits)
- Test unsubscribe flow

**Task 3: User acceptance testing**
- Send test sequence to internal stakeholders
- Verify content clarity and tone
- Check email deliverability (SPF, DKIM setup)
- Test on multiple email clients

## Affected Files

### New Files
- `cmd/chimney/email-templates/email-01-welcome.txt`
- `cmd/chimney/email-templates/email-02-usecases.txt`
- `cmd/chimney/email-templates/email-03-features.txt`
- `cmd/chimney/email-templates/email-04-comparison.txt`
- `cmd/chimney/email-templates/email-05-final.txt`
- `cmd/chimney/email-service.go`
- `cmd/chimney/email-service_test.go`
- `cmd/chimney/scheduler.go` (if using custom scheduler)
- `cmd/chimney/handler_unsubscribe.go`

### Modified Files
- `cmd/chimney/main.go` — add email service initialization and routes
- `cmd/chimney/config.go` — add email config section
- `docs/use-cases/README.md` — ensure use case docs are linked properly
- `docs/quickstart.md` — verify quickstart is complete for new users

## Acceptance Criteria

### Content Criteria
- [ ] All 5 emails are written and reviewed
- [ ] Each email has a clear single CTA
- [ ] All links point to valid documentation pages
- [ ] Unsubscribe link present in all emails
- [ ] Subject lines are clear and not spam-like
- [ ] Content passes legal review (CAN-SPAM compliance)

### Technical Criteria
- [ ] Email templates render correctly with sample data
- [ ] Email 1 sends within 5 minutes of trial signup
- [ ] Emails 2-5 send at correct intervals (±2 hours)
- [ ] No duplicate emails sent to same user
- [ ] Unsubscribe requests processed within 24 hours
- [ ] Error handling works (SMTP failures logged, retries scheduled)

### Integration Criteria
- [ ] chimney service starts without errors with email config
- [ ] Trial signup webhook triggers Email 1
- [ ] Database records email sends correctly
- [ ] Analytics tracking captures opens and clicks
- [ ] Support can view email history for a trial

### Success Metrics
- [ ] Open rate >40% across sequence
- [ ] Click rate >10% on primary CTAs
- [ ] Unsubscribe rate <5%
- [ ] Trial-to-paid conversion increases by baseline +20%

## Out of Scope

- Marketing automation platform integration (e.g., HubSpot, Customer.io) — future enhancement
- Dynamic content based on user behavior (e.g., "viewed pricing page") — future enhancement
- A/B testing of email variants — future enhancement
- SMS or in-app notifications — separate feature
- Email personalization beyond basic fields (name, trial ID, dates) — future enhancement
- Advanced segmentation (e.g., enterprise vs. SMB) — future enhancement
- Translation to non-English languages — future enhancement
- Email lead scoring or qualification — separate feature
