# Cloudroof Trial Offer Structure

This document defines the trial offer parameters for community outreach campaigns.

## Trial Parameters

### Default Trial Configuration

```yaml
TrialDuration: 30 days
NodeLimit: 5 nodes
Features:
  - Full mesh networking
  - NAT traversal
  - WireGuard encryption
  - DHT discovery
  - CLI interface
  - Basic support
```

### Extended Trial Configuration

For enterprise teams or larger deployments:

```yaml
TrialDuration: 60 days
NodeLimit: 20 nodes
Features:
  - All default features
  - Priority support
  - Setup assistance
  - Custom configuration guidance
```

## Trial Offer Structure by Community Type

### Kubernetes & Cloud Native Communities

**Value Proposition**: Mesh networking for multi-cluster Kubernetes deployments

```yaml
PromoCodeFormat: K8S{RANDOM}
TrialDuration: 30 days
NodeLimit: 10 nodes  # Kubernetes clusters typically need more nodes
Messaging:
  - Connect pods across clusters
  - Service mesh without control plane complexity
  - CNCF-aligned architecture
```

### Homelab & Self-Hosted Communities

**Value Proposition**: Connect home lab to cloud VPS securely

```yaml
PromoCodeFormat: LAB{RANDOM}
TrialDuration: 30 days
NodeLimit: 5 nodes
Messaging:
  - Homelab to VPS in 5 minutes
  - No public ports needed
  - Persistent IPs behind NAT
```

### DevOps & SRE Communities

**Value Proposition**: Ad-hoc mesh for infrastructure operations

```yaml
PromoCodeFormat: OPS{RANDOM}
TrialDuration: 45 days
NodeLimit: 10 nodes
Messaging:
  - Fleet management without VPN servers
  - Emergency access paths
  - Backup connectivity
```

### Networking & Security Communities

**Value Proposition**: Zero-trust networking with no coordination server

```yaml
PromoCodeFormat: NET{RANDOM}
TrialDuration: 30 days
NodeLimit: 5 nodes
Messaging:
  - WireGuard encryption
  - DHT discovery
  - No cloud trust boundary
```

## Trial Onboarding Flow

### Step 1: Sign Up with Promo Code

```bash
# User receives promo code from community post
# Example: K8SABCD1234E

# Visit cloudroof trial signup page
# Or use CLI:
wgmesh trial signup K8SABCD1234E
```

### Step 2: Account Creation

```yaml
RequiredFields:
  - Email (optional but recommended for support)
  - Promo code (validated against store)
  - Preferred mesh name (optional)

GeneratedOnSignup:
  - Account ID
  - Trial expiration date
  - Initial mesh secret
```

### Step 3: First Mesh Setup

```bash
# User generates mesh secret
wgmesh init --secret

# Or uses provided quickstart for their platform
# (Hetzner, AWS, homelab, etc.)
```

### Step 4: Trial Activation

**Definition**: Trial is considered "activated" when user successfully joins their second node to the mesh (indicating they've actually used the product).

```yaml
ActivationTrigger: Second node joins mesh
ActivationEvent: trial_activation
Tracking: Analytics logger event
```

### Step 5: Trial Conversion

**Definition**: Trial is "converted" when user subscribes to paid plan or converts to self-hosted.

```yaml
ConversionTrigger: Subscription purchase or self-hosted deployment
ConversionEvent: trial_conversion
Tracking: Analytics logger event
```

## Trial Extensions

### Automatic Extensions

Based on engagement and use case:

```yaml
HighEngagementExtension:
  Criteria:
    - 5+ nodes active in trial
    - 20+ days since signup
    - Active usage (daily peer connections)
  Extension: +14 days
  Triggered: Automatically via analytics

EnterpriseTrialExtension:
  Criteria:
    - User requests via support
    - Demonstrates enterprise use case
  Extension: +30 days
  Triggered: Manual approval
```

## Trial Metrics and KPIs

### Funnel Metrics

```yaml
Signup:
  Definition: User redeems promo code
  Target: 500 signups from 50 communities

Activation:
  Definition: User joins 2+ nodes to mesh
  Target: 40% activation rate

Conversion:
  Definition: User subscribes or self-hosts
  Target: 10% conversion rate from activated trials
```

### Community-Specific Targets

```yaml
KubernetesCommunities:
  SignupTarget: 150
  ActivationTarget: 35%
  ConversionTarget: 12%

HomelabCommunities:
  SignupTarget: 100
  ActivationTarget: 45%
  ConversionTarget: 8%

DevOpsCommunities:
  SignupTarget: 150
  ActivationTarget: 40%
  ConversionTarget: 10%

NetworkingCommunities:
  SignupTarget: 100
  ActivationTarget: 30%
  ConversionTarget: 15%
```

## Trial Communication Flow

### Welcome Email/Message

```
Subject: Your cloudroof trial is ready!

Hi [Name],

Your 30-day cloudroof trial is now active. Your trial code: [CODE]

To get started:
1. Generate a mesh secret: wgmesh init --secret
2. Join your first node: wgmesh join --secret "[SECRET]"
3. Join your second node to activate your trial

Need help? Reply to this message or join our Discord: [LINK]

Your trial expires on [DATE]. We'll send a reminder 7 days before.

Happy meshing!
The cloudroof team
```

### Activation Confirmation

```
Subject: Trial activated! 🎉

Great news - your trial is now active with 2 nodes in your mesh.

Next steps:
- Add more nodes: wgmesh join --secret "[SECRET]"
- Expose services: wgmesh expose [options]
- Check status: wgmesh status

Questions? Just reply!
```

### Expiration Reminder (7 days)

```
Subject: Trial expires in 7 days

Hi [Name],

Your trial expires on [DATE]. You currently have [X] nodes in your mesh.

To continue using cloudroof:
- Subscribe: [LINK]
- Self-host: [LINK to docs]

Need more time? Reply to this message and we'll extend your trial.
```

### Post-Trial Follow-up

```
Subject: How was your trial?

Hi [Name],

Your trial has ended. We'd love to hear your feedback:

[Feedback survey link]

If you'd like to continue using cloudroof, we have special offers for trial users.

Cheers,
The cloudroof team
```

## Special Offers for High-Performing Communities

### Top 10 Communities by Conversion

```yaml
Reward:
  - Extended 60-day trials for all members
  - Dedicated support channel
  - Case study opportunities
  - Potential partnership discussions
```

### Referral Bonus

```yaml
ForMembersOf:
  - Communities with 20+ trial signups
  - Communities with 15%+ conversion rate

Offer:
  - Additional 30 days per successful referral
  - Promo code: REF[COMMUNITY][RANDOM]
  - Tracking via existing promo system
```

---

## Implementation Status

- [X] Default trial parameters defined
- [X] Community-specific trial structures
- [X] Onboarding flow documented
- [X] Trial extension logic defined
- [X] Communication templates created
- [ ] Integration with promo code system (see `pkg/promo/`)
- [ ] Integration with analytics tracking (see `pkg/analytics/`)

Last updated: [Date]
