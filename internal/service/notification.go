package service

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"

	"github.com/prepmyapp/notification/internal/domain"
)

// EmailSender is the interface for sending emails.
type EmailSender interface {
	Send(ctx context.Context, to, subject, body string) error
	SendTemplate(ctx context.Context, to, templateID string, data map[string]interface{}) error
	SendHTML(ctx context.Context, to, subject, plainText, htmlContent string) error
}

// PushSender is the interface for sending push notifications.
type PushSender interface {
	Send(ctx context.Context, token, title, body string, data map[string]interface{}) error
	SendToUser(ctx context.Context, userID uuid.UUID, title, body string, data map[string]interface{}) error
}

// InAppNotifier is the interface for sending in-app notifications.
type InAppNotifier interface {
	Notify(ctx context.Context, userID uuid.UUID, notification *domain.Notification) error
}

// NotificationService orchestrates notification sending across all channels.
type NotificationService struct {
	notificationRepo domain.NotificationRepository
	deviceTokenRepo  domain.DeviceTokenRepository
	preferencesRepo  domain.PreferencesRepository
	emailSender      EmailSender
	pushSender       PushSender
	inAppNotifier    InAppNotifier
}

// NewNotificationService creates a new notification service.
func NewNotificationService(
	notificationRepo domain.NotificationRepository,
	deviceTokenRepo domain.DeviceTokenRepository,
	preferencesRepo domain.PreferencesRepository,
	emailSender EmailSender,
	pushSender PushSender,
	inAppNotifier InAppNotifier,
) *NotificationService {
	return &NotificationService{
		notificationRepo: notificationRepo,
		deviceTokenRepo:  deviceTokenRepo,
		preferencesRepo:  preferencesRepo,
		emailSender:      emailSender,
		pushSender:       pushSender,
		inAppNotifier:    inAppNotifier,
	}
}

// SendRequest represents a request to send notifications.
type SendRequest struct {
	UserID   uuid.UUID
	Email    string // Required for email channel
	Channels []domain.NotificationType
	Template string
	Title    string
	Body     string
	Data     map[string]interface{}
}

// Send sends notifications through the specified channels.
func (s *NotificationService) Send(ctx context.Context, req SendRequest) error {
	// Get user preferences (if preferencesRepo is available)
	var prefs *domain.NotificationPreferences
	if s.preferencesRepo != nil {
		var err error
		prefs, err = s.preferencesRepo.Get(ctx, req.UserID)
		if err != nil {
			// Use default preferences if not found
			prefs = domain.NewDefaultPreferences(req.UserID)
		}
	} else {
		prefs = domain.NewDefaultPreferences(req.UserID)
	}

	// Check quiet hours
	if prefs.IsInQuietHours() {
		// During quiet hours, only send critical notifications (like OTP)
		if req.Template != "otp_verification" && req.Template != "password_reset" {
			log.Printf("Skipping notification during quiet hours for user %s", req.UserID)
			return nil
		}
	}

	var errors []error

	for _, channel := range req.Channels {
		var err error

		switch channel {
		case domain.NotificationTypeEmail:
			if !prefs.EmailEnabled {
				log.Printf("Email notifications disabled for user %s", req.UserID)
				continue
			}
			err = s.sendEmail(ctx, req)

		case domain.NotificationTypePush:
			if !prefs.PushEnabled {
				log.Printf("Push notifications disabled for user %s", req.UserID)
				continue
			}
			err = s.sendPush(ctx, req)

		case domain.NotificationTypeInApp:
			err = s.sendInApp(ctx, req)
		}

		if err != nil {
			errors = append(errors, fmt.Errorf("%s: %w", channel, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("notification errors: %v", errors)
	}

	return nil
}

// sendEmail sends an email notification.
func (s *NotificationService) sendEmail(ctx context.Context, req SendRequest) error {
	if s.emailSender == nil {
		return fmt.Errorf("email sender not configured")
	}

	if req.Email == "" {
		return fmt.Errorf("email address required")
	}

	// Create notification record
	notification := domain.NewNotification(
		req.UserID,
		domain.NotificationTypeEmail,
		req.Template,
		req.Title,
		req.Body,
	)
	notification.Metadata = req.Data

	// Save to database
	if err := s.notificationRepo.Create(ctx, notification); err != nil {
		return fmt.Errorf("failed to create notification record: %w", err)
	}

	// Send email based on template type
	var err error
	switch req.Template {
	case "otp_verification":
		// Use styled OTP email template
		otp := ""
		if req.Data != nil {
			if otpVal, ok := req.Data["otp"]; ok {
				otp = fmt.Sprintf("%v", otpVal)
			}
		}
		htmlContent := generateOtpEmailHtml(otp)
		err = s.emailSender.SendHTML(ctx, req.Email, req.Title, req.Body, htmlContent)
	default:
		// For other emails, use simple send
		err = s.emailSender.Send(ctx, req.Email, req.Title, req.Body)
	}

	// Update status
	if err != nil {
		if statusErr := s.notificationRepo.UpdateStatus(ctx, notification.ID, domain.NotificationStatusFailed); statusErr != nil {
			log.Printf("failed to update notification status to failed: %v", statusErr)
		}
		return fmt.Errorf("failed to send email: %w", err)
	}

	if err := s.notificationRepo.UpdateStatus(ctx, notification.ID, domain.NotificationStatusSent); err != nil {
		log.Printf("failed to update notification status to sent: %v", err)
	}
	return nil
}

// sendPush sends a push notification to all user devices.
func (s *NotificationService) sendPush(ctx context.Context, req SendRequest) error {
	if s.pushSender == nil {
		return fmt.Errorf("push sender not configured")
	}

	// Create notification record
	notification := domain.NewNotification(
		req.UserID,
		domain.NotificationTypePush,
		req.Template,
		req.Title,
		req.Body,
	)
	notification.Metadata = req.Data

	if err := s.notificationRepo.Create(ctx, notification); err != nil {
		return fmt.Errorf("failed to create notification record: %w", err)
	}

	// Send push notification
	err := s.pushSender.SendToUser(ctx, req.UserID, req.Title, req.Body, req.Data)

	if err != nil {
		if statusErr := s.notificationRepo.UpdateStatus(ctx, notification.ID, domain.NotificationStatusFailed); statusErr != nil {
			log.Printf("failed to update notification status to failed: %v", statusErr)
		}
		return fmt.Errorf("failed to send push: %w", err)
	}

	if err := s.notificationRepo.UpdateStatus(ctx, notification.ID, domain.NotificationStatusSent); err != nil {
		log.Printf("failed to update notification status to sent: %v", err)
	}
	return nil
}

// sendInApp creates an in-app notification and broadcasts it via WebSocket.
func (s *NotificationService) sendInApp(ctx context.Context, req SendRequest) error {
	// Create notification record
	notification := domain.NewNotification(
		req.UserID,
		domain.NotificationTypeInApp,
		req.Template,
		req.Title,
		req.Body,
	)
	notification.Metadata = req.Data

	if err := s.notificationRepo.Create(ctx, notification); err != nil {
		return fmt.Errorf("failed to create notification record: %w", err)
	}

	// Mark as sent (in-app notifications are "sent" when stored)
	notification.MarkAsSent()
	if err := s.notificationRepo.UpdateStatus(ctx, notification.ID, domain.NotificationStatusSent); err != nil {
		log.Printf("failed to update notification status to sent: %v", err)
	}

	// Broadcast via WebSocket if available
	if s.inAppNotifier != nil {
		if err := s.inAppNotifier.Notify(ctx, req.UserID, notification); err != nil {
			// Log but don't fail - the notification is still stored
			log.Printf("Failed to broadcast in-app notification: %v", err)
		}
	}

	return nil
}

// GetNotifications retrieves notifications for a user.
func (s *NotificationService) GetNotifications(ctx context.Context, userID uuid.UUID, opts domain.ListOptions) ([]*domain.Notification, int64, error) {
	return s.notificationRepo.GetByUserID(ctx, userID, opts)
}

// GetNotification retrieves a single notification.
func (s *NotificationService) GetNotification(ctx context.Context, id uuid.UUID) (*domain.Notification, error) {
	return s.notificationRepo.GetByID(ctx, id)
}

// MarkAsRead marks a notification as read.
func (s *NotificationService) MarkAsRead(ctx context.Context, id uuid.UUID) error {
	return s.notificationRepo.MarkAsRead(ctx, id)
}

// MarkAllAsRead marks all notifications for a user as read.
func (s *NotificationService) MarkAllAsRead(ctx context.Context, userID uuid.UUID) error {
	return s.notificationRepo.MarkAllAsRead(ctx, userID)
}

// GetUnreadCount returns the count of unread notifications.
func (s *NotificationService) GetUnreadCount(ctx context.Context, userID uuid.UUID) (int64, error) {
	return s.notificationRepo.GetUnreadCount(ctx, userID)
}

// generateOtpEmailHtml generates a styled HTML email for OTP verification.
// Uses brand colors: Primary Navy #1E3A5F, Primary Cyan #7DD3FC
func generateOtpEmailHtml(otp string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>PrepMyApp Verification Code</title>
</head>
<body style="margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif; line-height: 1.6; color: #111827; background-color: #F9FAFB;">
    <div style="max-width: 600px; margin: 20px auto; background-color: #ffffff; border-radius: 16px; box-shadow: 0 4px 20px rgba(30, 58, 95, 0.1); overflow: hidden;">
        <!-- Header with Navy gradient -->
        <div style="background: linear-gradient(135deg, #1E3A5F 0%%, #2d4a6f 100%%); padding: 40px 20px; text-align: center;">
            <img src="https://prepmyapp.com/prepmyapp.png" alt="PrepMyApp" style="width: 64px; height: 64px; margin-bottom: 12px; border-radius: 12px;">
            <h1 style="font-size: 28px; font-weight: bold; color: #ffffff; margin: 0; letter-spacing: 1px;">PrepMyApp</h1>
            <p style="color: #7DD3FC; font-size: 14px; margin: 8px 0 0 0; font-weight: 500;">Automate Your Applications</p>
        </div>

        <!-- Main Content -->
        <div style="padding: 40px 30px; text-align: center;">
            <h2 style="font-size: 24px; font-weight: 600; color: #1E3A5F; margin: 0 0 16px 0;">Verification Code</h2>
            <p style="font-size: 16px; color: #6B7280; margin: 0 0 32px 0; line-height: 1.6;">
                We received a request to access your PrepMyApp account.<br>Use the code below to complete your sign-in.
            </p>

            <!-- OTP Code Box -->
            <div style="background: linear-gradient(135deg, #F9FAFB 0%%, #F3F4F6 100%%); border-radius: 12px; padding: 32px; margin: 0 0 32px 0; border: 2px solid #E5E7EB;">
                <p style="font-size: 42px; font-weight: bold; color: #1E3A5F; letter-spacing: 12px; margin: 0; font-family: 'SF Mono', 'Courier New', monospace;">%s</p>
                <p style="font-size: 12px; color: #9CA3AF; margin: 12px 0 0 0; text-transform: uppercase; letter-spacing: 2px; font-weight: 600;">Verification Code</p>
            </div>

            <!-- Expiry Notice -->
            <div style="background-color: #FEF3C7; border-radius: 8px; padding: 14px 20px; margin: 0 0 24px 0; display: inline-block;">
                <p style="font-size: 14px; color: #92400E; margin: 0; font-weight: 500;">
                    ⏱ This code expires in 5 minutes
                </p>
            </div>

            <!-- Security Notice -->
            <div style="background-color: #F0F9FF; border-left: 4px solid #7DD3FC; padding: 16px 20px; margin: 0 0 20px 0; text-align: left; border-radius: 0 8px 8px 0;">
                <p style="font-size: 14px; color: #1E3A5F; margin: 0;">
                    🔒 If you didn't request this code, please ignore this email. Your account remains secure.
                </p>
            </div>
        </div>

        <!-- Footer -->
        <div style="background-color: #1E3A5F; padding: 30px; text-align: center;">
            <div style="margin: 0 0 20px 0;">
                <a href="https://prepmyapp.com" style="color: #7DD3FC; text-decoration: none; font-size: 13px; margin: 0 12px;">Website</a>
                <span style="color: #4B5563;">|</span>
                <a href="https://prepmyapp.com/privacy" style="color: #7DD3FC; text-decoration: none; font-size: 13px; margin: 0 12px;">Privacy</a>
                <span style="color: #4B5563;">|</span>
                <a href="https://prepmyapp.com/terms" style="color: #7DD3FC; text-decoration: none; font-size: 13px; margin: 0 12px;">Terms</a>
                <span style="color: #4B5563;">|</span>
                <a href="mailto:info@prepmy.app" style="color: #7DD3FC; text-decoration: none; font-size: 13px; margin: 0 12px;">Support</a>
            </div>

            <p style="font-size: 12px; color: #9CA3AF; margin: 0 0 12px 0; line-height: 1.6;">
                This email was sent because you requested a verification code for PrepMyApp.
            </p>

            <p style="font-size: 11px; color: #6B7280; margin: 0;">
                © 2025 PrepMyApp, LLC · <a href="mailto:info@prepmy.app" style="color: #7DD3FC; text-decoration: none;">info@prepmy.app</a>
            </p>
        </div>
    </div>
</body>
</html>`, otp)
}
