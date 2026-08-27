# Polar.sh Product Configurations

**Document created:** 2026-06-18  
**Purpose:** Catalog product IDs and configurations for cloudroof.eu sponsor tiers and reference paying product

## Overview

This document tracks the Polar.sh product configurations used for the cloudroof.eu landing page sponsor tiers. These products represent different sponsorship levels for the wgmesh project.

## Cloudroof Tier Products

### Founding Member ($5/mo)

- **Product ID:** `3f5d75de-936b-49d8-a21b-4b79d9fd22c1`
- **Price:** $5/month
- **Polar.sh Checkout:** https://polar.sh/checkout?productId=3f5d75de-936b-49d8-a21b-4b79d9fd22c1
- **Positioning:** "Become a founding member"
- **Subscriber Count:** 0 (as of docs/index.html display)
- **Social Proof Display:** "0 founding members so far — be first"

**Benefits:**
- Your name on this dashboard — permanent recognition
- Discord/Matrix access — follow architecture decisions live
- Binding vote on roadmap priorities
- Early access to all releases

### Edge Node ($20/mo)

- **Product ID:** `1927e637-4cfd-4c94-8bee-c5518803bc89`
- **Price:** $20/month
- **Polar.sh Checkout:** https://polar.sh/checkout?productId=1927e637-4cfd-4c94-8bee-c5518803bc89
- **Positioning:** "Reserve your edge node"
- **Subscriber Count:** 0 (not explicitly displayed, inferred from tier pattern)
- **Social Proof Display:** No count displayed for this tier

**Benefits:**
- Everything in Founding Member
- cloudroof.eu edge node — beta access Q2 2026, you're in queue
- Private Discord/Matrix channel
- Logo on project README

### Mesh Operator ($100/mo)

- **Product ID:** `eb20683e-55ea-4354-9d8c-070e55a4eff5`
- **Price:** $100/month
- **Polar.sh Checkout:** https://polar.sh/checkout?productId=eb20683e-55ea-4354-9d8c-070e55a4eff5
- **Positioning:** "Become a mesh operator"
- **Subscriber Count:** 0 (not explicitly displayed, inferred from tier pattern)
- **Social Proof Display:** No count displayed for this tier

**Benefits:**
- Everything in Edge Node
- 5 free CDN nodes
- Direct support via Slack/email
- Custom feature requests
- Quarterly architecture review call

## Reference Paying Product

### Product 8e8e1c33

- **Product ID:** `8e8e1c33`
- **Price:** Unknown (requires manual Polar.sh dashboard verification)
- **Polar.sh Checkout:** https://polar.sh/checkout?productId=8e8e1c33
- **Positioning:** Unknown (requires investigation)
- **Subscriber Count:** Unknown (paying product, assumed >0)
- **Status:** Known paying product referenced in issue #759

**Analysis Required:**
- What type of product is this? (tool vs. sponsorship vs. service)
- How is it positioned?
- What are the benefits?
- Are benefits immediate or future-delivered?
- How is social proof displayed?
- What is the price point?
- What persona does it target?

## Verification Instructions

To verify and update product configurations:

1. Run the audit script:
   ```bash
   bash scripts/audit-polar-products.sh
   ```

2. Visit each Polar.sh checkout link manually

3. Document findings in:
   - `docs/product-8e8e1c33-analysis.md` (detailed analysis of paying product)
   - `docs/cloudroof-positioning-analysis.md` (gap analysis)
   - `docs/benefit-delivery-roadmap.md` (benefit timing and delivery)

## Key Observations

### Subscriber Counts
All three cloudroof tier products display **zero subscribers**:
- Founding Member explicitly shows "0 founding members so far — be first"
- Edge Node and Mesh Operator have no count displayed (pattern suggests 0)

### Positioning Language
Call-to-action language suggests sponsorship mental model:
- "Become a founding member"
- "Reserve your edge node"
- "Become a mesh operator"

This contrasts with typical product purchase language like "Buy now", "Subscribe", or "Purchase".

### Benefit Timing Analysis
- **Founding Member:** Mostly immediate benefits (recognition, access, voting)
- **Edge Node:** Mixed - immediate recognition + deferred beta access (Q2 2026)
- **Mesh Operator:** Mixed - immediate support + deferred CDN nodes (timeline unclear)

## Next Steps

1. **Manual Investigation:** Visit Polar.sh dashboards to verify subscriber counts and product positioning
2. **Product 8e8e1c33 Analysis:** Document characteristics of successful paying product
3. **Gap Analysis:** Compare successful vs. unsuccessful positioning strategies
4. **Benefit Delivery Roadmap:** Establish realistic timelines for deferred benefits
5. **A/B Testing:** Test alternative positioning and social proof strategies

## Related Documents

- [Issue #759 Specification](../pipeline-output/issue-759-spec.md)
- [Product 8e8e1c33 Analysis](product-8e8e1c33-analysis.md) (to be created)
- [Cloudroof Positioning Analysis](cloudroof-positioning-analysis.md) (to be created)
- [Benefit Delivery Roadmap](benefit-delivery-roadmap.md) (to be created)
