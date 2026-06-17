package nurture

import (
	"bytes"
	"fmt"
	"net/smtp"
	"os"
	"strings"
	"text/template"
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

// loadTemplates loads email templates
func (s *Sender) loadTemplates() error {
	templates := map[string]string{
		"trial-welcome": `Subject: {{.Subject}}

Hi {{.Email}},

Your cloudroof.eu trial is ready!

You're 14 days away from secure, managed edge ingress. Let's get started.

Step 1: Install wgmesh
{{range .InstallCmd}}
{{.}}{{end}}

Step 2: Register your edge service
{{range .ServiceCmd}}
{{.}}{{end}}

Step 3: Access your service
Your service is now available at: {{.ServiceURL}}

Questions? Reply to this email - we answer every one.

---
Unsubscribe: {{.UnsubscribeURL}}
`,
		"trial-day-1": `Subject: {{.Subject}}

Hi {{.Email}},

Ready to expose your first service?

With wgmesh, you can wire any edge service to the internet in minutes. Here's how:

1. Install wgmesh on your edge server
2. Run: wgmesh service register --name my-api --port 8080
3. Access your service instantly via HTTPS

No public IPs needed. No complex firewall config. Just secure mesh networking.

Try it now and let us know how it goes!

---
Unsubscribe: {{.UnsubscribeURL}}
`,
		"trial-day-3": `Subject: {{.Subject}}

Hi {{.Email}},

Pro tip: Use custom domains for your services.

Instead of remembering random endpoints, you can assign your own domain:

wgmesh service register --name my-api --port 8080 --domain api.example.com

Your service will be accessible at: https://api.example.com

This works with any DNS provider - wgmesh handles the TLS automatically.

---
Unsubscribe: {{.UnsubscribeURL}}
`,
		"trial-week-1": `Subject: {{.Subject}}

Hi {{.Email}},

Week 1 check-in: How's your trial going?

Have you been able to:
- Expose at least one service?
- Test the mesh connectivity?
- Explore custom domains?

If you're stuck, just reply to this email. We're happy to help debug any issues.

Your trial expires in 7 days. Let us know if you need more time!

---
Unsubscribe: {{.UnsubscribeURL}}
`,
		"trial-reminder": `Subject: {{.Subject}}

Hi {{.Email}},

3 days left on your trial!

Your cloudroof.eu trial expires soon. Here's what you can do:

1. Extend your trial: {{.ExtendURL}}
2. Upgrade to a paid plan: {{.PricingURL}}
3. Request a custom demo: reply to this email

We'd love to have you as a customer. Let us know what you need!

---
Unsubscribe: {{.UnsubscribeURL}}
`,
	}

	for name, content := range templates {
		tmpl, err := template.New(name).Parse(content)
		if err != nil {
			return fmt.Errorf("parsing template %s: %w", name, err)
		}
		s.templates[name] = tmpl
	}

	return nil
}

// SendTemplate sends a templated email
func (s *Sender) SendTemplate(to string, templateID string, data map[string]interface{}) error {
	// Get template
	tmpl, ok := s.templates[templateID]
	if !ok {
		return fmt.Errorf("template not found: %s", templateID)
	}

	// Render template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("rendering template: %w", err)
	}

	content := buf.String()

	// Extract subject line
	parts := strings.SplitN(content, "\n\n", 2)
	if len(parts) < 2 {
		return fmt.Errorf("invalid template format")
	}
	subject := strings.TrimPrefix(parts[0], "Subject: ")
	body := parts[1]

	// Send based on provider
	switch s.provider {
	case "smtp":
		return s.sendSMTP(to, subject, body)
	case "log":
		return s.sendLog(to, subject, body)
	default:
		return fmt.Errorf("unknown provider: %s", s.provider)
	}
}

// sendSMTP sends email via SMTP
func (s *Sender) sendSMTP(to, subject, body string) error {
	if s.smtpHost == "" {
		return fmt.Errorf("SMTP host not configured")
	}

	auth := smtp.PlainAuth("", s.smtpUser, s.smtpPass, s.smtpHost)
	addr := fmt.Sprintf("%s:%s", s.smtpHost, s.smtpPort)

	msg := fmt.Sprintf("To: %s\r\nFrom: %s\r\nReply-To: %s\r\nSubject: %s\r\n\r\n%s",
		to, s.from, s.replyTo, subject, body)

	return smtp.SendMail(addr, auth, s.from, []string{to}, []byte(msg))
}

// sendLog logs email to stdout (for testing)
func (s *Sender) sendLog(to, subject, body string) error {
	fmt.Printf("[EMAIL] To: %s | Subject: %s\n%s\n", to, subject, body)
	return nil
}

// Close closes the sender
func (s *Sender) Close() error {
	return nil
}
