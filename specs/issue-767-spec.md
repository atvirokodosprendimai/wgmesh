# Issue #767: Add web analytics tracking (Plausible/PostHog) to cloudroof.eu landing

## Classification
feature

## Problem Analysis

The cloudroof.eu landing page currently lacks web analytics tracking, making it impossible to measure:
- User traffic and visitor metrics
- Conversion funnel from landing page to sign-up
- Geographic distribution of visitors
- Referral sources and campaign effectiveness
- User engagement patterns (bounce rate, time on page, scroll depth)

The product team needs visibility into landing page performance to optimize marketing efforts and improve user acquisition.

### Current State
- cloudroof.eu is a static/marketing site (separate from wgmesh codebase)
- No analytics integration exists
- Privacy-conscious product requiring GDPR-compliant analytics

### Requirements
- Must be privacy-focused (GDPR compliant, no cookies)
- Must support self-hosted or EU-hosted options
- Must not impact page load performance significantly
- Must be deployable via existing infrastructure

## Proposed Approach

### Analytics Platform Selection
Evaluate and choose between:
1. **Plausible Analytics** (primary recommendation)
   - Privacy-focused, GDPR compliant by design
   - Lightweight (< 1 KB)
   - Self-hosted or cloud-hosted options (EU servers available)
   - Open source
   - No cookies, no personal data collection

2. **PostHog** (alternative)
   - More feature-rich (product analytics + session replay)
   - Self-hosted or cloud-hosted options
   - EU hosting available
   - Larger footprint (~5 KB compressed)
   - Risk: may be overkill for simple landing page analytics

### Implementation Steps

1. **Platform setup**
   - Create Plausible/PostHog account
   - Set up tracking domain property
   - Generate site ID / API key
   - Configure data retention (default 90 days recommended)

2. **Script integration**
   - Add analytics script to cloudroof.eu landing page
   - Implement as deferred/async script to avoid blocking render
   - Add to all landing page routes (home, pricing, about, etc.)
   - Test script loading and event firing

3. **Privacy compliance**
   - Add analytics disclosure to privacy policy
   - Implement cookie banner (if required) or note that no cookies are used
   - Ensure no PII is captured (check URL parameters, custom events)
   - Configure IP anonymization (enabled by default in Plausible)

4. **Custom events** (optional, Phase 2)
   - Track CTA clicks (sign-up buttons)
   - Track QR code scans
   - Track documentation link clicks
   - Track pricing tier clicks

5. **Dashboard setup**
   - Create main dashboard: visitors, bounce rate, visit duration
   - Create goals: CTA clicks, external link clicks
   - Set up weekly/monthly email reports for team

### Technical Implementation

For Plausible (recommended):
```html
<script defer data-domain="cloudroof.eu" src="https://plausible.io/js/script.js"></script>
```

For self-hosted Plausible:
```html
<script defer data-domain="cloudroof.eu" src="https://analytics.cloudroof.eu/js/script.js"></script>
```

For PostHog:
```html
<script>
    !function(t,e){var o,n,p,r;e.__SV||(window.posthog=e,e._i=[],e.init=function(i,s,a){function g(t,e){var o=e.split(".");2==o.length&&(t=t[o[0]],e=o[1]);t[e]=function(){t.push([e].concat(Array.prototype.slice.call(arguments,0)))}}(p=t.createElement("script")).type="text/javascript",p.crossOrigin="anonymous",p.async=!0,p.src=s.api_host+"/static/array.js",(r=t.getElementsByTagName("script")[0]).parentNode.insertBefore(p,r);var u=e;for(void 0!==a?u=e[a]=[]:a="posthog",u.people=u.people||[],u.toString=function(t){var e="posthog";return"posthog"!==a&&(e+="."+a),t||(e+=" (stub)"),e},u.people.toString=function(){return u.toString(1)+".people (stub)"},o="capture identify alias people.set people.set_once set_config register register_once unregister opt_out_capturing has_opted_out_capturing opt_in_capturing reset isFeatureEnabled onFeatureFlags getFeatureFlag getActiveFeatureFlags setFeatureFlagForSelf reloadFeatures".split(" ");for(n=0;n<o.length;n++)g(u,o[n]);e._i.push([i,s,a])},e.__SV=1)}(document,window.posthog||[]);
    posthog.init('{api_key}',{api_host:'https://app.posthog.com',loaded:function(posthog){posthog.identify()}})
</script>
```

## Acceptance Criteria

- [ ] Analytics script successfully integrated into cloudroof.eu landing page
- [ ] Script loads asynchronously without blocking page render (LCP impact < 100ms)
- [ ] Dashboard shows real-time visitor data within 5 minutes of deployment
- [ ] Core metrics reporting correctly: pageviews, unique visitors, bounce rate, visit duration
- [ ] Referrer sources tracked (direct, organic search, social, external links)
- [ ] Geographic data available (country/city level, no IP addresses)
- [ ] Privacy policy updated with analytics disclosure
- [ ] GDPR compliance verified (no cookies, no personal data, IP anonymization enabled)
- [ ] At least 2 custom goals configured (e.g., sign-up CTA clicks)
- [ ] Team access configured (at least 2 stakeholders can view dashboard)
- [ ] Documentation updated: analytics setup, dashboard access, event tracking guidelines

## Out of scope

- Integration with wgmesh backend infrastructure (this is frontend-only)
- Analytics for internal/admin panels
- A/B testing or experimentation framework
- Server-side analytics tracking
- Cross-domain tracking (e.g., if app.cloudroof.eu is separate)
- Real-time user session monitoring
- Personalization or behavioral targeting
- Cost analysis of paid analytics tiers (if not self-hosting)
- Historical data migration (analytics start from implementation date)
- Mobile app tracking (if mobile app exists separately)

## Notes

- Plausible Community Edition is free for up to 10K pageviews/month
- If self-hosting Plausible, requires PostgreSQL server and ClickHouse for analytics database
- Recommend starting with Plausible Cloud for simplicity, migrate to self-hosted if cost exceeds €9/month
- PostHog free tier: 1M events/month free, then may become costly at scale
- Script should be added to `<head>` section for best reliability
- Verify analytics work with ad blockers and privacy extensions (e.g., uBlock Origin)
