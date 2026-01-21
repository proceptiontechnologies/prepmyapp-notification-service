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

// NotificationRepository implements domain.NotificationRepository using PostgreSQL.
type NotificationRepository struct {
	pool *pgxpool.Pool
}

// NewNotificationRepository creates a new PostgreSQL notification repository.
func NewNotificationRepository(pool *pgxpool.Pool) *NotificationRepository {
	return &NotificationRepository{pool: pool}
}

// Create saves a new notification to the database.
func (r *NotificationRepository) Create(ctx context.Context, n *domain.Notification) error {
	metadata, err := json.Marshal(n.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		INSERT INTO notifications (id, user_id, type, channel, title, body, metadata, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err = r.pool.Exec(ctx, query,
		n.ID,
		n.UserID,
		n.Type,
		n.Channel,
		n.Title,
		n.Body,
		metadata,
		n.Status,
		n.CreatedAt,
		n.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create notification: %w", err)
	}

	return nil
}

// GetByID retrieves a notification by its ID.
func (r *NotificationRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Notification, error) {
	query := `
		SELECT id, user_id, type, channel, title, body, metadata, status, read_at, sent_at, created_at, updated_at
		FROM notifications
		WHERE id = $1
	`

	row := r.pool.QueryRow(ctx, query, id)
	return r.scanNotification(row)
}

// GetByUserID retrieves notifications for a specific user with pagination.
func (r *NotificationRepository) GetByUserID(ctx context.Context, userID uuid.UUID, opts domain.ListOptions) ([]*domain.Notification, int64, error) {
	// Build the query based on options
	baseQuery := `FROM notifications WHERE user_id = $1`
	args := []interface{}{userID}
	argIndex := 2

	if opts.Unread {
		baseQuery += " AND read_at IS NULL"
	}

	// Get total count
	countQuery := "SELECT COUNT(*) " + baseQuery
	var total int64
	err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count notifications: %w", err)
	}

	// Get paginated results
	selectQuery := `
		SELECT id, user_id, type, channel, title, body, metadata, status, read_at, sent_at, created_at, updated_at
	` + baseQuery + fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIndex, argIndex+1)

	args = append(args, opts.Limit, opts.Offset)

	rows, err := r.pool.Query(ctx, selectQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query notifications: %w", err)
	}
	defer rows.Close()

	var notifications []*domain.Notification
	for rows.Next() {
		n, err := r.scanNotificationFromRows(rows)
		if err != nil {
			return nil, 0, err
		}
		notifications = append(notifications, n)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating notifications: %w", err)
	}

	return notifications, total, nil
}

// UpdateStatus updates the status of a notification.
func (r *NotificationRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.NotificationStatus) error {
	query := `
		UPDATE notifications
		SET status = $2, updated_at = $3, sent_at = CASE WHEN $2 = 'sent' THEN $3 ELSE sent_at END
		WHERE id = $1
	`

	result, err := r.pool.Exec(ctx, query, id, status, time.Now())
	if err != nil {
		return fmt.Errorf("failed to update notification status: %w", err)
	}

	if result.RowsAffected() == 0 {
		return domain.NewErrNotFound("notification", id.String())
	}

	return nil
}

// MarkAsRead marks a notification as read.
func (r *NotificationRepository) MarkAsRead(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE notifications
		SET read_at = $2, updated_at = $2
		WHERE id = $1 AND read_at IS NULL
	`

	now := time.Now()
	result, err := r.pool.Exec(ctx, query, id, now)
	if err != nil {
		return fmt.Errorf("failed to mark notification as read: %w", err)
	}

	if result.RowsAffected() == 0 {
		return domain.NewErrNotFound("notification", id.String())
	}

	return nil
}

// MarkAllAsRead marks all notifications for a user as read.
func (r *NotificationRepository) MarkAllAsRead(ctx context.Context, userID uuid.UUID) error {
	query := `
		UPDATE notifications
		SET read_at = $2, updated_at = $2
		WHERE user_id = $1 AND read_at IS NULL
	`

	_, err := r.pool.Exec(ctx, query, userID, time.Now())
	if err != nil {
		return fmt.Errorf("failed to mark all notifications as read: %w", err)
	}

	return nil
}

// GetUnreadCount returns the count of unread notifications for a user.
func (r *NotificationRepository) GetUnreadCount(ctx context.Context, userID uuid.UUID) (int64, error) {
	query := `
		SELECT COUNT(*)
		FROM notifications
		WHERE user_id = $1 AND read_at IS NULL AND type = 'in_app'
	`

	var count int64
	err := r.pool.QueryRow(ctx, query, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get unread count: %w", err)
	}

	return count, nil
}

// DeleteOlderThan removes notifications older than the specified number of days.
func (r *NotificationRepository) DeleteOlderThan(ctx context.Context, days int) (int64, error) {
	query := `
		DELETE FROM notifications
		WHERE created_at < NOW() - INTERVAL '1 day' * $1
	`

	result, err := r.pool.Exec(ctx, query, days)
	if err != nil {
		return 0, fmt.Errorf("failed to delete old notifications: %w", err)
	}

	return result.RowsAffected(), nil
}

// scanNotification scans a single row into a Notification.
func (r *NotificationRepository) scanNotification(row pgx.Row) (*domain.Notification, error) {
	var n domain.Notification
	var metadata []byte

	err := row.Scan(
		&n.ID,
		&n.UserID,
		&n.Type,
		&n.Channel,
		&n.Title,
		&n.Body,
		&metadata,
		&n.Status,
		&n.ReadAt,
		&n.SentAt,
		&n.CreatedAt,
		&n.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, domain.NewErrNotFound("notification", "")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan notification: %w", err)
	}

	if err := json.Unmarshal(metadata, &n.Metadata); err != nil {
		n.Metadata = make(map[string]interface{})
	}

	return &n, nil
}

// scanNotificationFromRows scans from pgx.Rows into a Notification.
func (r *NotificationRepository) scanNotificationFromRows(rows pgx.Rows) (*domain.Notification, error) {
	var n domain.Notification
	var metadata []byte

	err := rows.Scan(
		&n.ID,
		&n.UserID,
		&n.Type,
		&n.Channel,
		&n.Title,
		&n.Body,
		&metadata,
		&n.Status,
		&n.ReadAt,
		&n.SentAt,
		&n.CreatedAt,
		&n.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to scan notification: %w", err)
	}

	if err := json.Unmarshal(metadata, &n.Metadata); err != nil {
		n.Metadata = make(map[string]interface{})
	}

	return &n, nil
}
