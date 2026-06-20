package nurture

import (
	"bytes"
	"fmt"
	"net/smtp"
	"os"
	"text/template"
	"time"
)

// Sender handles email sending
type Sender struct {
	from      string
	replyTo   string
	provider  string
	smtpHost  string
	smtpPort  string
	smtpUser  string
	smtpPass  string
	templates map[string]*template.Template
}

// TemplateData holds data for email template rendering
type TemplateData struct {
	TrialID             string
	Email               string
	ReplyTo             string
	StartDate           time.Time
	ExpiryDate          time.Time
	DaysRemaining       int
	UpgradeLink         string
	UnsubscribeLink     string
	QuickstartLink      string
	UseCasesLink        string
	FeaturesLink        string
	FAQLink             string
	TroubleshootingLink string
}

// NewSender creates a new email sender from environment config
func NewSender() (*Sender, error) {
	s := &Sender{
		from:      getEnv("NURTURE_FROM", "noreply@cloudroof.eu"),
		replyTo:   getEnv("NURTURE_REPLY_TO", "support@cloudroof.eu"),
		provider:  getEnv("NURTURE_EMAIL_PROVIDER", "log"),
		smtpHost:  getEnv("SMTP_HOST", ""),
		smtpPort:  getEnv("SMTP_PORT", "587"),
		smtpUser:  getEnv("SMTP_USER", ""),
		smtpPass:  getEnv("SMTP_PASS", ""),
		templates: make(map[string]*template.Template),
	}

	// Load templates
	if err := s.loadTemplates(); err != nil {
		return nil, fmt.Errorf("loading templates: %w", err)
	}

	return s, nil
}

// getEnv retrieves an environment variable with a default
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// loadTemplates loads email templates from the templates directory
func (s *Sender) loadTemplates() error {
	templates := map[string]string{
		"email-01-welcome":    welcomeTemplate,
		"email-02-usecases":   usecasesTemplate,
		"email-03-features":   featuresTemplate,
		"email-04-comparison": comparisonTemplate,
		"email-05-final":      finalTemplate,
	}

	for id, content := range templates {
		tmpl, err := template.New(id).Parse(content)
		if err != nil {
			return fmt.Errorf("parsing template %s: %w", id, err)
		}
		s.templates[id] = tmpl
	}

	return nil
}

// SendEmail sends a single email
func (s *Sender) SendEmail(templateID, subject string, data *TemplateData) error {
	tmpl, ok := s.templates[templateID]
	if !ok {
		return fmt.Errorf("template not found: %s", templateID)
	}

	var body bytes.Buffer
	if err := tmpl.Execute(&body, data); err != nil {
		return fmt.Errorf("rendering template: %w", err)
	}

	switch s.provider {
	case "smtp":
		return s.sendSMTP(subject, data.Email, body.Bytes())
	case "log":
		return s.sendLog(subject, data.Email, body.Bytes())
	default:
		return fmt.Errorf("unknown email provider: %s", s.provider)
	}
}

// sendSMTP sends email via SMTP
func (s *Sender) sendSMTP(subject string, to string, body []byte) error {
	if s.smtpHost == "" {
		return fmt.Errorf("SMTP host not configured")
	}

	// Compose message
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		s.from, to, subject, body)

	// Send
	addr := fmt.Sprintf("%s:%s", s.smtpHost, s.smtpPort)
	auth := smtp.PlainAuth("", s.smtpUser, s.smtpPass, s.smtpHost)

	return smtp.SendMail(addr, auth, s.from, []string{to}, []byte(msg))
}

// sendLog logs email content (for testing/dry-run mode)
func (s *Sender) sendLog(subject string, to string, body []byte) error {
	fmt.Printf("[EMAIL] To: %s | Subject: %s\n", to, subject)
	fmt.Printf("[EMAIL] Body:\n%s\n", string(body))
	return nil
}

// welcomeTemplate is the Day 0 welcome email
const welcomeTemplate = `Welcome to Cloudroof - Let's get your mesh running

Hi {{.Email}},

Your trial is now active! Here's everything you need to get started.

QUICK START
==========
Get 3 nodes connected in under 15 minutes:
{{.QuickstartLink}}

First milestone: Get your first mesh running with 3 connected nodes.
Expected time: 15 minutes

YOUR TRIAL DETAILS
================
Trial ID: {{.TrialID}}
Started: {{.StartDate}}
Expires: {{.ExpiryDate}}
Days remaining: {{.DaysRemaining}}

NEED HELP?
=========
Quickstart Guide: {{.QuickstartLink}}
Documentation: docs.cloudroof.eu
Email Support: {{.ReplyTo}}

Let's get your mesh running!

---
To stop receiving these emails, unsubscribe: {{.UnsubscribeLink}}
`

// usecasesTemplate is the Day 2 use cases email
const usecasesTemplate = `How's your mesh coming along?

Hi {{.Email}},

By now you should have your first mesh running. Here are 3 powerful ways to use Cloudroof:

1. SITE-TO-SITE VPN
Connect your office network to cloud resources securely.
Full guide: {{.UseCasesLink}}/hybrid-site-to-site

2. MULTI-CLOUD CONNECTIVITY
Link AWS, GCP, and Azure VPCs without public IPs.
Full guide: {{.UseCasesLink}}/multi-cloud

3. REMOTE TEAM ACCESS
Give developers secure access to internal services.
Full guide: {{.UseCasesLink}}/remote-dev-team

SUCCESS METRIC
============
Teams with 5+ connected nodes see 90% fewer connectivity issues.

Your trial: {{.DaysRemaining}} days remaining
Explore all use cases: {{.UseCasesLink}}

---
To stop receiving these emails, unsubscribe: {{.UnsubscribeLink}}
`

// featuresTemplate is the Day 5 advanced features email
const featuresTemplate = `Beyond basic meshing: Advanced Cloudroof features

Hi {{.Email}},

You've got the basics working. Here's what makes Cloudroof powerful:

ADVANCED FEATURES
================
• NAT Traversal: Automatic UDP hole-punching
• Relay Fallback: Peers connect even behind symmetric NAT
• DHT Discovery: Serverless peer finding
• Gossip Protocol: In-mesh peer broadcast

MANAGED INGRESS (Paid Tier)
==========================
Upgrade for:
• CDN-backed ingress with global edge deployment
• Automated xDS sync for Envoy integration
• Priority support and SLA guarantees

Try managed ingress: {{.FeaturesLink}}

Your trial: {{.DaysRemaining}} days remaining
Upgrade now: {{.UpgradeLink}}

---
To stop receiving these emails, unsubscribe: {{.UnsubscribeLink}}
`

// comparisonTemplate is the Day 12 comparison email
const comparisonTemplate = `Your trial is halfway done - Here's what you're missing

Hi {{.Email}},

Your trial expires in {{.DaysRemaining}} days. Here's what you get when you upgrade:

FREE TIER
========
• Self-managed mesh networking
• Community support
• Basic DHT discovery
• Up to 10 nodes

PAID TIER
=========
• Everything in Free, plus:
• Managed ingress (CDN-backed)
• Priority email support
• 99.9% SLA guarantee
• Unlimited nodes
• Advanced analytics and monitoring
• xDS/Envoy integration

UPGRADE BONUS
============
Upgrade this week and get 20% off your first month.
Use code: TRIALUPGRADE

Upgrade now: {{.UpgradeLink}}

Your trial expires: {{.ExpiryDate}}
Don't lose your mesh continuity!

---
To stop receiving these emails, unsubscribe: {{.UnsubscribeLink}}
`

// finalTemplate is the Day 18 final reminder email
const finalTemplate = `Last chance to upgrade your Cloudroof trial

Hi {{.Email}},

Your trial expires in {{.DaysRemaining}} days.

KEEP YOUR MESH RUNNING
=====================
Upgrade now to ensure uninterrupted service:
{{.UpgradeLink}}

STAY ON FREE TIER
================
If you prefer the free tier, here are resources for success:

• Evaluation Checklist: docs.cloudroof.eu/evaluation-checklist.md
• FAQ: {{.FAQLink}}
• Troubleshooting Guide: {{.TroubleshootingLink}}

EXTEND YOUR TRIAL
=================
Need more time? Contact us to request a trial extension:
{{.ReplyTo}}

Trial ID: {{.TrialID}}
Expires: {{.ExpiryDate}}

Upgrade now: {{.UpgradeLink}}

---
To stop receiving these emails, unsubscribe: {{.UnsubscribeLink}}
`
