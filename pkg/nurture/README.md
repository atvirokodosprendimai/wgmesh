# Email Nurture Sequence

This package implements a 5-email drip campaign for Cloudroof trial signups with onboarding tips, mesh-use cases, and paid-tier upgrade CTAs.

## Overview

The nurture sequence is designed to guide trial users through successful onboarding and demonstrate the value of Cloudroof to improve trial-to-paid conversion rates.

## Email Sequence

| Day | Email | Subject | Purpose |
|-----|-------|---------|---------|
| 0 | Welcome | "Welcome to Cloudroof - Let's get your mesh running" | Quick start guidance and first milestone |
| 2 | Use Cases | "How's your mesh coming along?" | Show real-world use cases |
| 5 | Features | "Beyond basic meshing: Advanced Cloudroof features" | Tease advanced features and managed ingress |
| 12 | Comparison | "Your trial is halfway done - Here's what you're missing" | Free vs Paid tier comparison |
| 18 | Final | "Last chance to upgrade your Cloudroof trial" | Final reminder and resources |

## Usage

```go
import "github.com/atvirokodosprendimai/wgmesh/pkg/nurture"

// Create sender (reads from environment)
sender, err := nurture.NewSender()
if err != nil {
    log.Fatalf("Failed to create sender: %v", err)
}

// Prepare template data
data := &nurture.TemplateData{
    TrialID:          "trial-123",
    Email:            "user@example.com",
    ReplyTo:          "support@cloudroof.eu",
    StartDate:        time.Now(),
    ExpiryDate:       time.Now().Add(14 * 24 * time.Hour),
    DaysRemaining:    14,
    UpgradeLink:      "https://cloudroof.eu/upgrade",
    UnsubscribeLink:  "https://cloudroof.eu/unsubscribe?trial=trial-123",
    QuickstartLink:   "https://docs.cloudroof.eu/quickstart",
    UseCasesLink:     "https://docs.cloudroof.eu/use-cases",
    FeaturesLink:     "https://docs.cloudroof.eu/features",
    FAQLink:          "https://docs.cloudroof.eu/faq",
    TroubleshootingLink: "https://docs.cloudroof.eu/troubleshooting",
}

// Send welcome email
err = sender.SendEmail("email-01-welcome", "Welcome to Cloudroof", data)
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `NURTURE_FROM` | `noreply@cloudroof.eu` | From email address |
| `NURTURE_REPLY_TO` | `support@cloudroof.eu` | Reply-to address |
| `NURTURE_EMAIL_PROVIDER` | `log` | Email provider (`smtp` or `log`) |
| `SMTP_HOST` | (empty) | SMTP server hostname |
| `SMTP_PORT` | `587` | SMTP server port |
| `SMTP_USER` | (empty) | SMTP username |
| `SMTP_PASS` | (empty) | SMTP password |

## Email Provider Modes

### Log Mode (Default)
Emails are printed to stdout for testing and dry-run:
```
[EMAIL] To: user@example.com | Subject: Welcome
[EMAIL] Body: ...
```

### SMTP Mode
Emails are sent via SMTP. Requires SMTP_HOST, SMTP_USER, and SMTP_PASS.

## Template Data

All templates support the following fields:

- `TrialID` - Unique trial identifier
- `Email` - Recipient email address
- `ReplyTo` - Support email for replies
- `StartDate` - Trial start date (time.Time)
- `ExpiryDate` - Trial expiry date (time.Time)
- `DaysRemaining` - Days until trial expires (int)
- `UpgradeLink` - URL to upgrade to paid tier
- `UnsubscribeLink` - URL to unsubscribe from emails
- `QuickstartLink` - URL to quickstart guide
- `UseCasesLink` - URL to use cases documentation
- `FeaturesLink` - URL to features documentation
- `FAQLink` - URL to FAQ
- `TroubleshootingLink` - URL to troubleshooting guide

## Getting Pending Emails

To determine which emails should be sent for a trial:

```go
createdAt := trial.CreatedAt
alreadySent := map[string]bool{
    "trial_welcome": true, // Already sent
}

pending := nurture.GetPendingEmails(createdAt, alreadySent)
for _, email := range pending {
    // Send email
    sender.SendEmail(email.TemplateID, email.Subject, data)
}
```

## Compliance

All emails include:
- Clear unsubscribe link (CAN-SPAM compliant)
- Physical postal address (if required)
- Accurate subject lines
- Cloudroof branding in footer

## Documentation References

Email templates reference the following documentation files (all exist in `docs/`):

- `docs/quickstart.md` - Quick start guide
- `docs/use-cases/README.md` - Use cases hub
- `docs/use-cases/hybrid-site-to-site.md` - Site-to-site VPN guide
- `docs/use-cases/multi-cloud.md` - Multi-cloud guide
- `docs/use-cases/remote-dev-team.md` - Remote team VPN guide
- `docs/FAQ.md` - Frequently asked questions
- `docs/troubleshooting.md` - Troubleshooting guide
- `docs/evaluation-checklist.md` - Evaluation checklist

## Testing

Run tests with:
```bash
go test ./pkg/nurture/... -v
```

Tests cover:
- Sender initialization with default and custom config
- Pending email calculation based on trial age
- Email sending with log and SMTP providers
- Template rendering with all placeholders
- Nurture sequence structure and uniqueness

## Design Decisions

1. **Text-only templates**: HTML versions omitted for MVP (spec note: "start with text-only")
2. **Log provider by default**: Safe for testing, prevents accidental sends
3. **Hardcoded templates**: Templates embedded in code for simplicity (spec: "create 5 template files" was for chimney, which was extracted)
4. **Unsubscribe in all emails**: CAN-SPAM compliance requirement
5. **Single CTA per email**: Each email focuses on one primary action
