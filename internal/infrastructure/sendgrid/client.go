package sendgrid

import (
	"context"
	"fmt"

	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

// Client wraps the SendGrid API client.
type Client struct {
	client    *sendgrid.Client
	fromEmail string
	fromName  string
}

// Config holds SendGrid configuration.
type Config struct {
	APIKey    string
	FromEmail string
	FromName  string
}

// NewClient creates a new SendGrid client.
func NewClient(cfg Config) *Client {
	return &Client{
		client:    sendgrid.NewSendClient(cfg.APIKey),
		fromEmail: cfg.FromEmail,
		fromName:  cfg.FromName,
	}
}

// Send sends a simple email with subject and body.
func (c *Client) Send(ctx context.Context, to, subject, body string) error {
	from := mail.NewEmail(c.fromName, c.fromEmail)
	toEmail := mail.NewEmail("", to)

	// Create message with both plain text and HTML
	message := mail.NewSingleEmail(from, subject, toEmail, body, body)

	response, err := c.client.Send(message)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	if response.StatusCode >= 400 {
		return fmt.Errorf("sendgrid error: status %d, body: %s", response.StatusCode, response.Body)
	}

	return nil
}

// SendTemplate sends an email using a SendGrid dynamic template.
func (c *Client) SendTemplate(ctx context.Context, to, templateID string, data map[string]interface{}) error {
	from := mail.NewEmail(c.fromName, c.fromEmail)
	toEmail := mail.NewEmail("", to)

	message := mail.NewV3Mail()
	message.SetFrom(from)
	message.SetTemplateID(templateID)

	personalization := mail.NewPersonalization()
	personalization.AddTos(toEmail)

	// Add dynamic template data
	for key, value := range data {
		personalization.SetDynamicTemplateData(key, value)
	}

	message.AddPersonalizations(personalization)

	response, err := c.client.Send(message)
	if err != nil {
		return fmt.Errorf("failed to send template email: %w", err)
	}

	if response.StatusCode >= 400 {
		return fmt.Errorf("sendgrid error: status %d, body: %s", response.StatusCode, response.Body)
	}

	return nil
}

// SendHTML sends an email with HTML content.
func (c *Client) SendHTML(ctx context.Context, to, subject, plainText, htmlContent string) error {
	from := mail.NewEmail(c.fromName, c.fromEmail)
	toEmail := mail.NewEmail("", to)

	message := mail.NewSingleEmail(from, subject, toEmail, plainText, htmlContent)

	response, err := c.client.Send(message)
	if err != nil {
		return fmt.Errorf("failed to send HTML email: %w", err)
	}

	if response.StatusCode >= 400 {
		return fmt.Errorf("sendgrid error: status %d, body: %s", response.StatusCode, response.Body)
	}

	return nil
}

// SendOTP sends an OTP verification email.
func (c *Client) SendOTP(ctx context.Context, to, otp string) error {
	subject := "Your PrepMyApp Verification Code"
	body := fmt.Sprintf(`Your verification code is: %s

This code will expire in 10 minutes.

If you didn't request this code, please ignore this email.

- The PrepMyApp Team
© 2025 PrepMyApp LLC`, otp)

	htmlBody := fmt.Sprintf(`
<div style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto;">
  <h2 style="color: #1E3A5F;">Your Verification Code</h2>
  <p style="font-size: 32px; font-weight: bold; color: #1E3A5F; letter-spacing: 8px;">%s</p>
  <p style="color: #666;">This code will expire in 10 minutes.</p>
  <p style="color: #999; font-size: 12px;">If you didn't request this code, please ignore this email.</p>
  <hr style="border: none; border-top: 1px solid #eee; margin: 20px 0;">
  <p style="color: #999; font-size: 12px;">- The PrepMyApp Team</p>
  <p style="color: #999; font-size: 11px;">© 2025 PrepMyApp LLC</p>
</div>`, otp)

	return c.SendHTML(ctx, to, subject, body, htmlBody)
}

// SendWelcome sends a welcome email to new users.
func (c *Client) SendWelcome(ctx context.Context, to, name string) error {
	subject := "Welcome to PrepMyApp!"
	body := fmt.Sprintf(`Hi %s,

Welcome to PrepMyApp! We're excited to help you land your dream job faster.

PrepMyApp streamlines your job application process with intelligent form automation and tracking, so you can focus on what matters most - preparing for interviews and advancing your career.

Get started by:
1. Completing your profile
2. Adding your first application
3. Using our browser extension for seamless form filling

If you have any questions, feel free to reach out to our support team at info@prepmy.app.

Best regards,
The PrepMyApp Team

© 2025 PrepMyApp LLC`, name)

	return c.Send(ctx, to, subject, body)
}
