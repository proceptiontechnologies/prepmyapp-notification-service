package domain

import (
	"context"

	"github.com/google/uuid"
)

// ListOptions provides pagination and filtering for list queries.
type ListOptions struct {
	Limit  int
	Offset int
	Unread bool // If true, only return unread notifications
}

// NotificationRepository defines the interface for notification persistence.
// In Go, interfaces are implicitly implemented - any struct that has these
// methods satisfies this interface (no "implements" keyword needed).
type NotificationRepository interface {
	// Create saves a new notification to the database.
	Create(ctx context.Context, notification *Notification) error

	// GetByID retrieves a notification by its ID.
	GetByID(ctx context.Context, id uuid.UUID) (*Notification, error)

	// GetByUserID retrieves notifications for a specific user with pagination.
	// Returns the notifications and total count for pagination.
	GetByUserID(ctx context.Context, userID uuid.UUID, opts ListOptions) ([]*Notification, int64, error)

	// UpdateStatus updates the status of a notification.
	UpdateStatus(ctx context.Context, id uuid.UUID, status NotificationStatus) error

	// MarkAsRead marks a notification as read.
	MarkAsRead(ctx context.Context, id uuid.UUID) error

	// MarkAllAsRead marks all notifications for a user as read.
	MarkAllAsRead(ctx context.Context, userID uuid.UUID) error

	// GetUnreadCount returns the count of unread notifications for a user.
	GetUnreadCount(ctx context.Context, userID uuid.UUID) (int64, error)

	// DeleteOlderThan removes notifications older than the specified duration.
	// Useful for cleanup jobs.
	DeleteOlderThan(ctx context.Context, days int) (int64, error)
}

// DeviceTokenRepository defines the interface for device token persistence.
type DeviceTokenRepository interface {
	// Create saves a new device token.
	Create(ctx context.Context, token *DeviceToken) error

	// GetByUserID retrieves all active device tokens for a user.
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]*DeviceToken, error)

	// GetByToken retrieves a device token by its token string.
	GetByToken(ctx context.Context, token string) (*DeviceToken, error)

	// Deactivate marks a device token as inactive.
	Deactivate(ctx context.Context, token string) error

	// Delete removes a device token.
	Delete(ctx context.Context, token string) error
}

// PreferencesRepository defines the interface for user preferences persistence.
type PreferencesRepository interface {
	// Get retrieves preferences for a user. Returns default preferences if none exist.
	Get(ctx context.Context, userID uuid.UUID) (*NotificationPreferences, error)

	// Upsert creates or updates preferences for a user.
	Upsert(ctx context.Context, prefs *NotificationPreferences) error
}

// ErrNotFound is returned when a requested entity doesn't exist.
// In Go, errors are values - we define custom errors as variables.
type ErrNotFound struct {
	Entity string
	ID     string
}

func (e *ErrNotFound) Error() string {
	return e.Entity + " not found: " + e.ID
}

// NewErrNotFound creates a new not found error.
func NewErrNotFound(entity, id string) *ErrNotFound {
	return &ErrNotFound{Entity: entity, ID: id}
}
