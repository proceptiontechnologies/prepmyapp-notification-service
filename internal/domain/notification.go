package domain

import (
	"time"

	"github.com/google/uuid"
)

// NotificationType defines the delivery channel for a notification.
// In Go, we use custom types with constants (like enums in other languages).
type NotificationType string

const (
	NotificationTypeEmail NotificationType = "email"
	NotificationTypePush  NotificationType = "push"
	NotificationTypeInApp NotificationType = "in_app"
)

// NotificationStatus tracks the lifecycle of a notification.
type NotificationStatus string

const (
	NotificationStatusPending   NotificationStatus = "pending"
	NotificationStatusSending   NotificationStatus = "sending"
	NotificationStatusSent      NotificationStatus = "sent"
	NotificationStatusDelivered NotificationStatus = "delivered"
	NotificationStatusFailed    NotificationStatus = "failed"
)

// Notification is the core domain entity.
// It represents a single notification to be delivered to a user.
type Notification struct {
	ID        uuid.UUID              `json:"id"`
	UserID    uuid.UUID              `json:"user_id"`
	Type      NotificationType       `json:"type"`
	Channel   string                 `json:"channel"` // e.g., "otp", "alert", "marketing"
	Title     string                 `json:"title"`
	Body      string                 `json:"body"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Status    NotificationStatus     `json:"status"`
	ReadAt    *time.Time             `json:"read_at,omitempty"`
	SentAt    *time.Time             `json:"sent_at,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// NewNotification creates a new notification with sensible defaults.
// This is a constructor pattern in Go (since Go doesn't have constructors).
func NewNotification(userID uuid.UUID, notifType NotificationType, channel, title, body string) *Notification {
	now := time.Now()
	return &Notification{
		ID:        uuid.New(),
		UserID:    userID,
		Type:      notifType,
		Channel:   channel,
		Title:     title,
		Body:      body,
		Metadata:  make(map[string]interface{}),
		Status:    NotificationStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// MarkAsSent updates the notification status to sent.
func (n *Notification) MarkAsSent() {
	now := time.Now()
	n.Status = NotificationStatusSent
	n.SentAt = &now
	n.UpdatedAt = now
}

// MarkAsDelivered updates the notification status to delivered.
func (n *Notification) MarkAsDelivered() {
	n.Status = NotificationStatusDelivered
	n.UpdatedAt = time.Now()
}

// MarkAsFailed updates the notification status to failed.
func (n *Notification) MarkAsFailed() {
	n.Status = NotificationStatusFailed
	n.UpdatedAt = time.Now()
}

// MarkAsRead marks an in-app notification as read.
func (n *Notification) MarkAsRead() {
	now := time.Now()
	n.ReadAt = &now
	n.UpdatedAt = now
}

// IsRead returns true if the notification has been read.
func (n *Notification) IsRead() bool {
	return n.ReadAt != nil
}

// DeviceToken represents a registered device for push notifications.
type DeviceToken struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Token     string    `json:"token"`
	Platform  string    `json:"platform"` // "ios", "android", "web"
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NewDeviceToken creates a new device token.
func NewDeviceToken(userID uuid.UUID, token, platform string) *DeviceToken {
	now := time.Now()
	return &DeviceToken{
		ID:        uuid.New(),
		UserID:    userID,
		Token:     token,
		Platform:  platform,
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// NotificationPreferences stores user preferences for notifications.
type NotificationPreferences struct {
	UserID          uuid.UUID       `json:"user_id"`
	EmailEnabled    bool            `json:"email_enabled"`
	PushEnabled     bool            `json:"push_enabled"`
	ChannelSettings map[string]bool `json:"channel_settings,omitempty"` // Per-channel preferences
	QuietHoursStart *time.Time      `json:"quiet_hours_start,omitempty"`
	QuietHoursEnd   *time.Time      `json:"quiet_hours_end,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// NewDefaultPreferences creates preferences with all notifications enabled.
func NewDefaultPreferences(userID uuid.UUID) *NotificationPreferences {
	now := time.Now()
	return &NotificationPreferences{
		UserID:          userID,
		EmailEnabled:    true,
		PushEnabled:     true,
		ChannelSettings: make(map[string]bool),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

// IsChannelEnabled checks if a specific channel is enabled for the user.
func (p *NotificationPreferences) IsChannelEnabled(channel string) bool {
	// If channel-specific setting exists, use it
	if enabled, exists := p.ChannelSettings[channel]; exists {
		return enabled
	}
	// Default to enabled if no specific setting
	return true
}

// IsInQuietHours checks if current time is within quiet hours.
func (p *NotificationPreferences) IsInQuietHours() bool {
	if p.QuietHoursStart == nil || p.QuietHoursEnd == nil {
		return false
	}

	now := time.Now()
	currentTime := now.Hour()*60 + now.Minute()
	startTime := p.QuietHoursStart.Hour()*60 + p.QuietHoursStart.Minute()
	endTime := p.QuietHoursEnd.Hour()*60 + p.QuietHoursEnd.Minute()

	// Handle overnight quiet hours (e.g., 22:00 - 07:00)
	if startTime > endTime {
		return currentTime >= startTime || currentTime < endTime
	}

	return currentTime >= startTime && currentTime < endTime
}
