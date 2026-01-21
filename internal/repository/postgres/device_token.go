package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/prepmyapp/notification/internal/domain"
)

// DeviceTokenRepository implements domain.DeviceTokenRepository using PostgreSQL.
type DeviceTokenRepository struct {
	pool *pgxpool.Pool
}

// NewDeviceTokenRepository creates a new PostgreSQL device token repository.
func NewDeviceTokenRepository(pool *pgxpool.Pool) *DeviceTokenRepository {
	return &DeviceTokenRepository{pool: pool}
}

// Create saves a new device token (upsert - update if token exists).
func (r *DeviceTokenRepository) Create(ctx context.Context, token *domain.DeviceToken) error {
	query := `
		INSERT INTO device_tokens (id, user_id, token, platform, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (token) DO UPDATE SET
			user_id = EXCLUDED.user_id,
			platform = EXCLUDED.platform,
			is_active = true,
			updated_at = EXCLUDED.updated_at
	`

	_, err := r.pool.Exec(ctx, query,
		token.ID,
		token.UserID,
		token.Token,
		token.Platform,
		token.IsActive,
		token.CreatedAt,
		token.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create device token: %w", err)
	}

	return nil
}

// GetByUserID retrieves all active device tokens for a user.
func (r *DeviceTokenRepository) GetByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.DeviceToken, error) {
	query := `
		SELECT id, user_id, token, platform, is_active, created_at, updated_at
		FROM device_tokens
		WHERE user_id = $1 AND is_active = true
		ORDER BY created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query device tokens: %w", err)
	}
	defer rows.Close()

	var tokens []*domain.DeviceToken
	for rows.Next() {
		token, err := r.scanDeviceToken(rows)
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating device tokens: %w", err)
	}

	return tokens, nil
}

// GetByToken retrieves a device token by its token string.
func (r *DeviceTokenRepository) GetByToken(ctx context.Context, token string) (*domain.DeviceToken, error) {
	query := `
		SELECT id, user_id, token, platform, is_active, created_at, updated_at
		FROM device_tokens
		WHERE token = $1
	`

	row := r.pool.QueryRow(ctx, query, token)

	var dt domain.DeviceToken
	err := row.Scan(
		&dt.ID,
		&dt.UserID,
		&dt.Token,
		&dt.Platform,
		&dt.IsActive,
		&dt.CreatedAt,
		&dt.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, domain.NewErrNotFound("device_token", token)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan device token: %w", err)
	}

	return &dt, nil
}

// Deactivate marks a device token as inactive.
func (r *DeviceTokenRepository) Deactivate(ctx context.Context, token string) error {
	query := `
		UPDATE device_tokens
		SET is_active = false, updated_at = $2
		WHERE token = $1
	`

	result, err := r.pool.Exec(ctx, query, token, time.Now())
	if err != nil {
		return fmt.Errorf("failed to deactivate device token: %w", err)
	}

	if result.RowsAffected() == 0 {
		return domain.NewErrNotFound("device_token", token)
	}

	return nil
}

// Delete removes a device token.
func (r *DeviceTokenRepository) Delete(ctx context.Context, token string) error {
	query := `DELETE FROM device_tokens WHERE token = $1`

	result, err := r.pool.Exec(ctx, query, token)
	if err != nil {
		return fmt.Errorf("failed to delete device token: %w", err)
	}

	if result.RowsAffected() == 0 {
		return domain.NewErrNotFound("device_token", token)
	}

	return nil
}

// scanDeviceToken scans a row into a DeviceToken.
func (r *DeviceTokenRepository) scanDeviceToken(rows pgx.Rows) (*domain.DeviceToken, error) {
	var dt domain.DeviceToken

	err := rows.Scan(
		&dt.ID,
		&dt.UserID,
		&dt.Token,
		&dt.Platform,
		&dt.IsActive,
		&dt.CreatedAt,
		&dt.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to scan device token: %w", err)
	}

	return &dt, nil
}
