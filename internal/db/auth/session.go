package auth

import (
	"context"
	"fmt"

	"github.com/StevenWeathers/thunderdome-planning-poker/internal/db"

	"github.com/StevenWeathers/thunderdome-planning-poker/thunderdome"
)

// CreateSession creates a new user authenticated session
func (d *Service) CreateSession(ctx context.Context, userID string, enabled bool) (string, error) {
	sessionID, err := db.RandomBase64String(32)
	if err != nil {
		return "", err
	}

	// Pending MFA sessions keep their original seven-day expiration until verification.
	if _, sessionErr := d.DB.ExecContext(ctx, `
		INSERT INTO thunderdome.user_session (session_id, user_id, disabled, expire_date)
		VALUES ($1, $2, $3, NOW() + CASE WHEN $3 THEN INTERVAL '7 days' ELSE $4::integer * INTERVAL '1 second' END);
		`,
		sessionID,
		userID,
		!enabled,
		thunderdome.SessionMaxAgeSeconds,
	); sessionErr != nil {
		return "", fmt.Errorf("create user session query error: %v", sessionErr)
	}

	return sessionID, nil
}

// EnableSession enables a user authenticated session
func (d *Service) EnableSession(ctx context.Context, sessionID string) error {
	result, err := d.DB.ExecContext(ctx, `
		UPDATE thunderdome.user_session
		SET disabled = false, expire_date = NOW() + $2::integer * INTERVAL '1 second'
		WHERE session_id = $1 AND disabled = true AND NOW() < expire_date;
		`,
		sessionID,
		thunderdome.SessionMaxAgeSeconds,
	)
	if err != nil {
		return fmt.Errorf("enable user session query error: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get enabled session row count: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("session is missing, expired, or already enabled")
	}

	return nil
}

// GetSessionUserByID gets a user session by sessionID
func (d *Service) GetSessionUserByID(ctx context.Context, sessionID string) (*thunderdome.User, error) {
	user := &thunderdome.User{}

	err := d.DB.QueryRowContext(ctx, `
		SELECT
        u.id,
        u.name,
        u.email,
        u.type,
        u.avatar,
		u.verified,
        u.notifications_enabled,
        COALESCE(u.country, ''),
        COALESCE(u.locale, ''),
        COALESCE(u.company, ''),
        COALESCE(u.job_title, ''),
        COALESCE(u.picture, ''),
        u.created_date,
        u.updated_date,
        u.last_active
    FROM thunderdome.user_session us
    LEFT JOIN thunderdome.users u ON u.id = us.user_id
	WHERE us.session_id = $1 AND us.disabled = false AND NOW() < us.expire_date`,
		sessionID,
	).Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		&user.Type,
		&user.Avatar,
		&user.Verified,
		&user.NotificationsEnabled,
		&user.Country,
		&user.Locale,
		&user.Company,
		&user.JobTitle,
		&user.Picture,
		&user.CreatedDate,
		&user.UpdatedDate,
		&user.LastActive,
	)
	if err != nil {
		return nil, fmt.Errorf("get session user query error: %v", err)
	}

	user.GravatarHash = db.CreateGravatarHash(user.Email)

	return user, nil
}

// DeleteSession deletes a user authenticated session
func (d *Service) DeleteSession(ctx context.Context, sessionID string) error {
	if _, sessionErr := d.DB.ExecContext(ctx, `
		DELETE FROM thunderdome.user_session WHERE session_id = $1;
		`,
		sessionID,
	); sessionErr != nil {
		return fmt.Errorf("delete user session query error: %v", sessionErr)
	}

	return nil
}
