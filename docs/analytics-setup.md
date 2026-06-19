# Web Analytics Setup for wgmesh.dev

## Overview

wgmesh.dev uses three privacy-focused analytics platforms to track landing page performance:

1. **Plausible Analytics** - Primary analytics (privacy-focused, GDPR compliant)
2. **PostHog** - Product analytics and event tracking
3. **OpenPanel** - Self-hosted analytics backup

All platforms are configured with IP anonymization enabled and no cookies are used, ensuring GDPR compliance without requiring consent banners.

## Platform Configuration

### Plausible Analytics

**Script Location:** `<head>` section of all landing pages  
**Domain:** `wgmesh.dev`  
**Script URL:** `https://plausible.io/js/script.js`  
**Features:**
- Pageview tracking
- Referrer sources
- Geographic data (country/city level)
- Custom events (CTA clicks, external link clicks)
- No cookies, IP anonymization enabled by default

**Dashboard Access:** [https://plausible.io](https://plausible.io)  
**Data Retention:** 90 days (default)

### PostHog

**Script Location:** `<head>` section of all landing pages  
**API Key:** `phc_YOUR_API_KEY_HERE` (replace with actual key)  
**API Host:** `https://app.posthog.com`  
**Features:**
- Product analytics
- Custom event tracking
- Funnel analysis
- Session recording (disabled, only basic events tracked)
- IP anonymization enabled

**Dashboard Access:** [https://app.posthog.com](https://app.posthog.com)  
**Free Tier:** 1M events/month

### OpenPanel

**Script Location:** `<head>` section of all landing pages  
**Client ID:** `80920d68-b57d-4c7b-baf6-3518495a3739`  
**API URL:** `https://counter.hackrsvalv.com/api`  
**Features:**
- Self-hosted analytics
- Pageview tracking
- Outgoing link tracking
- Custom event tracking

**Dashboard Access:** [https://counter.hackrsvalv.com](https://counter.hackrsvalv.com)

## Custom Events

### CTA Click Events

All pricing tier CTA buttons are tracked with the following properties:

```javascript
{
  event: 'cta_click',
  tier_id: 'founding-member' | 'edge-node' | 'mesh-operator',
  tier_name: '$5 Founding Member' | '$20 Edge Node' | '$100 Mesh Operator',
  location: 'pricing_section'
}
```

**Tracked across:** Plausible, PostHog, OpenPanel

### External Link Click Events

GitHub and Discord links are tracked with the following properties:

```javascript
{
  event: 'external_link_click',
  destination: '<url>'
}
```

**Tracked across:** Plausible, PostHog, OpenPanel

## Privacy Compliance

### GDPR Compliance

✅ **No cookies used** - All platforms use cookieless tracking  
✅ **IP anonymization enabled** - No personal IP addresses stored  
✅ **No PII collected** - Only anonymous aggregates  
✅ **Privacy disclosure** - Visible in footer with detailed information  
✅ **Opt-out available** - Users can block with privacy extensions or ad blockers  
✅ **Data retention limited** - 90 days default for Plausible  

### No Cookie Banner Required

Because we use cookieless tracking and don't collect personal data, no cookie consent banner is required under GDPR. This is confirmed by:

- Plausible: GDPR compliant by design (no cookies, no personal data)
- PostHog: IP anonymization enabled, no cookies for basic tracking
- OpenPanel: Self-hosted, no cookies

## Dashboard Setup

### Plausible Goals

Configure the following goals in Plausible:

1. **CTA Clicks**
   - Type: Custom Event
   - Event Name: `cta_click`
   - Funnel: Track which pricing tiers get most clicks

2. **External Link Clicks**
   - Type: Custom Event
   - Event Name: `external_link_click`
   - Funnel: Track which external resources are most popular

### PostHog Dashboards

Create the following dashboards:

1. **Landing Page Performance**
   - Unique visitors
   - Pageviews
   - Bounce rate
   - Session duration
   - Top referrers

2. **Conversion Funnel**
   - Landing page view → CTA click → Checkout initiated
   - Breakdown by pricing tier

3. **Geographic Distribution**
   - Visitors by country
   - Top cities

4. **Referral Sources**
   - Direct traffic
   - Organic search
   - Social media
   - External links

### Email Reports

Configure weekly/monthly email reports for:
- Pageview summary
- Top referral sources
- CTA click performance
- Geographic distribution

## Event Tracking Guidelines

### Adding New Custom Events

To add new custom events:

1. **PostHog:**
   ```javascript
   window.posthog.capture('event_name', {
     property1: 'value1',
     property2: 'value2'
   });
   ```

2. **Plausible:**
   ```javascript
   window.plausible('event_name', { props: {
     property1: 'value1',
     property2: 'value2'
   }});
   ```

3. **OpenPanel:**
   ```javascript
   window.op('event', 'event_name', {
     property1: 'value1',
     property2: 'value2'
   });
   ```

### Best Practices

- Use snake_case for event names
- Include relevant properties for filtering
- Don't include PII in event properties
- Test events in development before deploying
- Document custom events in this file

## Performance Impact

Analytics scripts are loaded with `defer` and `async` attributes to minimize performance impact:

- **Plausible:** < 1 KB gzipped
- **PostHog:** ~5 KB compressed
- **OpenPanel:** < 2 KB

**LCP Impact:** < 100ms (well within acceptable range)

## Migration to Self-Hosted Plausible

If traffic exceeds 10K pageviews/month, consider migrating to self-hosted Plausible:

1. Deploy Plausible instance (requires PostgreSQL + ClickHouse)
2. Update script URL to `https://analytics.wgmesh.dev/js/script.js`
3. Configure data retention (90 days recommended)
4. Migrate existing data (export from Plausible Cloud)

**Cost Consideration:** Plausible Cloud costs €9/month for up to 10K pageviews. Self-hosting becomes cost-effective beyond ~50K pageviews/month.

## Troubleshooting

### Analytics Not Showing Data

1. Check browser console for JavaScript errors
2. Verify scripts are loading (Network tab)
3. Check ad blocker settings (may block analytics)
4. Verify API keys and domains are correct
5. Test in incognito mode with extensions disabled

### Events Not Firing

1. Check if analytics libraries are loaded: `window.plausible`, `window.posthog`, `window.op`
2. Verify event names match dashboard configuration
3. Check browser console for errors
4. Test with console: `window.posthog.capture('test_event')`

### Geographic Data Inaccurate

- Geographic data is approximate (city/country level)
- Based on IP address lookup with anonymization
- Some visitors may appear from VPN/Proxy locations

## Team Access

### Plausible

- Invite team members via Settings → Team
- Roles: Owner, Admin, Viewer
- Recommended: Give stakeholders Viewer access

### PostHog

- Invite team members via Settings → Members
- Roles: Administrator, Member, Analyst
- Recommended: Give stakeholders Analyst access

### OpenPanel

- Self-hosted, configure user access via admin panel
- Contact administrator for access requests

## Resources

- [Plausible Documentation](https://plausible.io/docs)
- [PostHog Documentation](https://posthog.com/docs)
- [OpenPanel Documentation](https://openpanel.dev)
- [GDPR Compliance Guide](https://plausible.io/privacy-friendly-web-analytics)

## Changelog

- **2024-06-19:** Initial analytics integration with Plausible, PostHog, and OpenPanel
- **2024-06-19:** Added CTA click tracking for pricing tiers
- **2024-06-19:** Added external link click tracking
- **2024-06-19:** Added privacy disclosure to landing pages
