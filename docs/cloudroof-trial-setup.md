# cloudroof.eu Trial System Setup Guide

This guide explains how to configure and run the cloudroof.eu trial signup and nurture email system.

## Overview

The trial system consists of three main components:

1. **Landing Page** (`public/index.html`) - Email capture form for trial signups
2. **Trial Storage** (`pkg/trial/`) - Stores trial signups and email logs
3. **Nurture Sequence** (`pkg/nurture/`) - Sends timed email sequence to trial users

## Architecture

Since Lighthouse was decoupled to a separate repository, the trial system is implemented as a standalone service within wgmesh. The system uses:

- **File-based storage** (`pkg/trial/store.go`) - JSON file persistence (can be upgraded to SQLite)
- **Email sending** (`pkg/nurture/sender.go`) - SMTP with environment-based configuration
- **Nurture worker** - Background process that sends emails at scheduled intervals

## Configuration

### Environment Variables

Configure the following environment variables:

```bash
# Email provider (log, smtp)
NURTURE_EMAIL_PROVIDER=smtp

# SMTP settings (for smtp provider)
SMTP_HOST=smtp.example.com
SMTP_PORT=587
SMTP_USER=apikey
SMTP_PASS=your-sendgrid-api-key

# From/reply-to addresses
NURTURE_FROM=noreply@cloudroof.eu
NURTURE_REPLY_TO=support@cloudroof.eu
```

### Email Providers

The system supports multiple email providers via the `NURTURE_EMAIL_PROVIDER` variable:

#### 1. Log Provider (Testing)
```bash
NURTURE_EMAIL_PROVIDER=log
```
Prints emails to stdout for testing. No actual emails sent.

#### 2. SMTP Provider (Production)
```bash
NURTURE_EMAIL_PROVIDER=smtp
SMTP_HOST=smtp.sendgrid.net
SMTP_PORT=587
SMTP_USER=apikey
SMTP_PASS=SG.your-api-key
```
Sends emails via SMTP. Compatible with SendGrid, Mailgun, AWS SES, or any SMTP server.

## Deployment

### Step 1: Build the Trial Worker

```bash
go build -o trial-worker ./cmd/trial-worker
```

### Step 2: Create Data Directory

```bash
mkdir -p /var/lib/wgmesh/trials
chmod 700 /var/lib/wgmesh/trials
```

### Step 3: Configure Environment

Create `/etc/wgmesh/trial.env`:

```bash
export NURTURE_EMAIL_PROVIDER=smtp
export SMTP_HOST=smtp.sendgrid.net
export SMTP_PORT=587
export SMTP_USER=apikey
export SMTP_PASS=SG.your-api-key
export NURTURE_FROM=noreply@cloudroof.eu
export NURTURE_REPLY_TO=support@cloudroof.eu
export TRIAL_STORE_PATH=/var/lib/wgmesh/trials/trials.json
```

### Step 4: Run the Trial Worker

```bash
source /etc/wgmesh/trial.env
./trial-worker
```

### Step 5: Set Up systemd Service

Create `/etc/systemd/system/wgmesh-trial.service`:

```ini
[Unit]
Description=wgmesh Trial Nurture Worker
After=network.target

[Service]
Type=simple
User=wgmesh
EnvironmentFile=/etc/wgmesh/trial.env
ExecStart=/usr/local/bin/trial-worker
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
systemctl enable wgmesh-trial
systemctl start wgmesh-trial
```

## Landing Page Deployment

The landing page at `public/index.html` is a static HTML file with JavaScript for form submission. Deploy it to your web server:

### Option 1: Serve from wgmesh Binary

Add to your existing web server or serve as a static file handler.

### Option 2: CDN Deployment

Upload to a CDN (Cloudflare, CloudFront) for global distribution.

### Option 3: Separate Web Server

Use nginx or Apache to serve the static files:

```nginx
server {
    listen 80;
    server_name cloudroof.eu;

    root /var/www/cloudroof.eu;
    index index.html;

    location / {
        try_files $uri $uri/ =404;
    }

    # Proxy API requests to trial signup handler
    location /api/trial-signup {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
    }
}
```

## API Endpoint

The trial signup API endpoint should be implemented in your web service. Example Go handler:

```go
package main

import (
    "encoding/json"
    "fmt"
    "net/http"
    "time"

    "github.com/atvirokodosprendimai/wgmesh/pkg/nurture"
    "github.com/atvirokodosprendimai/wgmesh/pkg/trial"
)

type TrialSignupRequest struct {
    Email  string `json:"email"`
    Source string `json:"source"`
}

type TrialSignupResponse struct {
    Success bool   `json:"success"`
    Message string `json:"message"`
    TrialID string `json:"trial_id,omitempty"`
    Exists  bool   `json:"exists"`
}

func handleTrialSignup(w http.ResponseWriter, r *http.Request) {
    var req TrialSignupRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request", 400)
        return
    }

    // Validate email format
    if !isValidEmail(req.Email) {
        respondJSON(w, TrialSignupResponse{
            Success: false,
            Message: "Invalid email format",
        })
        return
    }

    // Initialize trial store
    store, err := trial.NewFileStore("/var/lib/wgmesh/trials/trials.json")
    if err != nil {
        http.Error(w, "Storage error", 500)
        return
    }
    defer store.Close()

    // Check for existing trial
    if store.Exists(req.Email) {
        respondJSON(w, TrialSignupResponse{
            Success: true,
            Exists:  true,
            Message: "You already have an active trial. Check your inbox.",
        })
        return
    }

    // Generate trial ID and store
    trialID := generateTrialID()
    newTrial := &trial.Trial{
        ID:        trialID,
        Email:     req.Email,
        Source:    req.Source,
        CreatedAt: time.Now(),
        Status:    "pending",
    }

    if err := store.Create(newTrial); err != nil {
        http.Error(w, "Storage error", 500)
        return
    }

    // Trigger welcome email (async)
    go sendWelcomeEmail(newTrial)

    respondJSON(w, TrialSignupResponse{
        Success: true,
        Message: "Check your inbox for trial setup instructions",
        TrialID: trialID,
    })
}

func isValidEmail(email string) bool {
    // Basic email validation
    return len(email) > 3 && strings.Contains(email, "@")
}

func generateTrialID() string {
    return fmt.Sprintf("%d", time.Now().UnixNano())
}

func sendWelcomeEmail(tr *trial.Trial) {
    sender, err := nurture.NewSender()
    if err != nil {
        fmt.Printf("Error creating sender: %v\n", err)
        return
    }
    defer sender.Close()

    data := map[string]interface{}{
        "Subject":       "Welcome to cloudroof.eu",
        "Email":         tr.Email,
        "InstallCmd":    []string{"curl -sSL https://get.wgmesh.dev | sh"},
        "ServiceCmd":    []string{"wgmesh service register --name my-api --port 8080"},
        "ServiceURL":    "https://my-api.cloudroof.eu",
        "UnsubscribeURL": fmt.Sprintf("https://cloudroof.eu/unsubscribe?email=%s", tr.Email),
    }

    if err := sender.SendTemplate(tr.Email, "trial-welcome", data); err != nil {
        fmt.Printf("Error sending welcome email: %v\n", err)
    }
}
```

## Nurture Email Sequence

The system sends 5 emails over 14 days:

| Day | Email | Purpose |
|-----|-------|---------|
| 0 | Welcome | Trial setup instructions |
| 1 | Day 1 Tip | Guide to expose first service |
| 3 | Day 3 Feature | Custom domain feature reveal |
| 7 | Week 1 Check-in | Help offer and engagement |
| 11 | 3-day reminder | Expiration urgency and upgrade |

## Email Templates

Templates are defined in `pkg/nurture/sender.go` as Go text templates. To customize:

1. Edit the template strings in `loadTemplates()`
2. Rebuild and redeploy the trial worker

Template variables available:
- `{{.Subject}}` - Email subject line
- `{{.Email}}` - Recipient email
- `{{.TrialID}}` - Trial ID
- `{{.InstallCmd}}` - Installation command(s)
- `{{.ServiceCmd}}` - Service registration command(s)
- `{{.ServiceURL}}` - Example service URL
- `{{.UnsubscribeURL}}` - Unsubscribe link
- `{{.ExtendURL}}` - Trial extension link
- `{{.PricingURL}}` - Pricing page link

## Monitoring

### Check Trial Stats

Query the trial store directly:

```bash
# Total trials
jq '.trials | length' /var/lib/wgmesh/trials/trials.json

# Active trials
jq '.trials | map(select(.status == "pending" or .status == "active")) | length' /var/lib/wgmesh/trials/trials.json

# Converted trials
jq '.trials | map(select(.status == "converted")) | length' /var/lib/wgmesh/trials/trials.json
```

### Check Email Logs

```bash
# Emails sent for a trial
jq '.logs["trial-id-here"]' /var/lib/wgmesh/trials/trials.json
```

### Monitor Worker Logs

```bash
journalctl -u wgmesh-trial -f
```

## Troubleshooting

### Emails Not Sending

1. Check environment variables are set: `systemctl show wgmesh-trial --environment`
2. Verify SMTP credentials: Test with `NURTURE_EMAIL_PROVIDER=log` first
3. Check worker logs: `journalctl -u wgmesh-trial -n 100`

### Trial Signups Failing

1. Check web server error logs
2. Verify API endpoint is accessible
3. Check file permissions on trial store directory

### Trial Store Corruption

The file store uses atomic writes (temp file + rename). If corruption occurs:

1. Stop the worker: `systemctl stop wgmesh-trial`
2. Restore from backup: `cp /var/lib/wgmesh/trials/trials.json.backup /var/lib/wgmesh/trials/trials.json`
3. Restart worker: `systemctl start wgmesh-trial`

## Security Considerations

1. **Email Credentials**: Store SMTP credentials in environment files with `chmod 600` permissions
2. **Trial Data**: File store contains email addresses; protect with `chmod 700` on directory
3. **Unsubscribe Links**: Include unsubscribe links in all emails to comply with anti-spam laws
4. **Rate Limiting**: Add rate limiting to trial signup API to prevent abuse
5. **Email Validation**: Validate email format and check for duplicates before storage

## Next Steps

- Set up SPF/DKIM records for your sending domain to improve deliverability
- Add tracking pixels to monitor email open rates
- Integrate with payment processing for trial-to-production conversion
- Add A/B testing for landing page and email templates
- Set up analytics to track conversion funnel

## Support

For issues or questions:
- GitHub: https://github.com/atvirokodosprendimai/wgmesh
- Email: support@cloudroof.eu
