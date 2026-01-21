package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/prepmyapp/notification/internal/domain"
)

// PreferencesRepository implements domain.PreferencesRepository using PostgreSQL.
type PreferencesRepository struct {
	pool *pgxpool.Pool
}

// NewPreferencesRepository creates a new PostgreSQL preferences repository.
func NewPreferencesRepository(pool *pgxpool.Pool) *PreferencesRepository {
	return &PreferencesRepository{pool: pool}
}

// Get retrieves preferences for a user. Returns default preferences if none exist.
func (r *PreferencesRepository) Get(ctx context.Context, userID uuid.UUID) (*domain.NotificationPreferences, error) {
	query := `
		SELECT user_id, email_enabled, push_enabled, channels,
		       quiet_hours_start, quiet_hours_end, created_at, updated_at
		FROM notification_preferences
		WHERE user_id = $1
	`

	var prefs domain.NotificationPreferences
	var channels []byte
	var quietStart, quietEnd *time.Time

	err := r.pool.QueryRow(ctx, query, userID).Scan(
		&prefs.UserID,
		&prefs.EmailEnabled,
		&prefs.PushEnabled,
		&channels,
		&quietStart,
		&quietEnd,
		&prefs.CreatedAt,
		&prefs.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		// Return default preferences if none exist
		return domain.NewDefaultPreferences(userID), nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get preferences: %w", err)
	}

	// Parse channel settings
	prefs.ChannelSettings = make(map[string]bool)
	if err := json.Unmarshal(channels, &prefs.ChannelSettings); err != nil {
		prefs.ChannelSettings = make(map[string]bool)
	}

	prefs.QuietHoursStart = quietStart
	prefs.QuietHoursEnd = quietEnd

	return &prefs, nil
}

// Upsert creates or updates preferences for a user.
func (r *PreferencesRepository) Upsert(ctx context.Context, prefs *domain.NotificationPreferences) error {
	channels, err := json.Marshal(prefs.ChannelSettings)
	if err != nil {
		return fmt.Errorf("failed to marshal channel settings: %w", err)
	}

	query := `
		INSERT INTO notification_preferences
			(user_id, email_enabled, push_enabled, channels,
			 quiet_hours_start, quiet_hours_end, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (user_id) DO UPDATE SET
			email_enabled = EXCLUDED.email_enabled,
			push_enabled = EXCLUDED.push_enabled,
			channels = EXCLUDED.channels,
			quiet_hours_start = EXCLUDED.quiet_hours_start,
			quiet_hours_end = EXCLUDED.quiet_hours_end,
			updated_at = EXCLUDED.updated_at
	`

	now := time.Now()
	prefs.UpdatedAt = now
	if prefs.CreatedAt.IsZero() {
		prefs.CreatedAt = now
	}

	_, err = r.pool.Exec(ctx, query,
		prefs.UserID,
		prefs.EmailEnabled,
		prefs.PushEnabled,
		channels,
		prefs.QuietHoursStart,
		prefs.QuietHoursEnd,
		prefs.CreatedAt,
		prefs.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to upsert preferences: %w", err)
	}

	return nil
}
