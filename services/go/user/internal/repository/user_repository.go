package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/opus-casino/user/internal/domain"
)

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func (r *UserRepository) GetUserByID(ctx context.Context, id int64) (*domain.User, error) {
	query := `
		SELECT id, uuid, email, username, first_name, last_name, phone, date_of_birth,
		       country_code, currency_code, status, kyc_level, language, timezone,
		       address, city, postal_code, referral_code, created_at, updated_at, last_login_at, metadata
		FROM users WHERE id = $1 AND deleted_at IS NULL
	`
	user := &domain.User{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&user.ID, &user.UUID, &user.Email, &user.Username,
		&user.FirstName, &user.LastName, &user.Phone, &user.DateOfBirth,
		&user.CountryCode, &user.CurrencyCode, &user.Status, &user.KYCLevel,
		&user.Language, &user.Timezone, &user.Address, &user.City,
		&user.PostalCode, &user.ReferralCode, &user.CreatedAt, &user.UpdatedAt,
		&user.LastLoginAt, &user.Metadata,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return user, nil
}

func (r *UserRepository) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `
		SELECT id, uuid, email, username, first_name, last_name, phone, date_of_birth,
		       country_code, currency_code, status, kyc_level, language, timezone,
		       address, city, postal_code, referral_code, created_at, updated_at, last_login_at, metadata
		FROM users WHERE email = $1 AND deleted_at IS NULL
	`
	user := &domain.User{}
	err := r.pool.QueryRow(ctx, query, email).Scan(
		&user.ID, &user.UUID, &user.Email, &user.Username,
		&user.FirstName, &user.LastName, &user.Phone, &user.DateOfBirth,
		&user.CountryCode, &user.CurrencyCode, &user.Status, &user.KYCLevel,
		&user.Language, &user.Timezone, &user.Address, &user.City,
		&user.PostalCode, &user.ReferralCode, &user.CreatedAt, &user.UpdatedAt,
		&user.LastLoginAt, &user.Metadata,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return user, nil
}

func (r *UserRepository) UpdateUser(ctx context.Context, req *domain.UpdateUserRequest) (*domain.User, error) {
	query := `
		UPDATE users SET
			username = COALESCE($2, username),
			first_name = COALESCE($3, first_name),
			last_name = COALESCE($4, last_name),
			phone = COALESCE($5, phone),
			date_of_birth = COALESCE($6, date_of_birth),
			address = COALESCE($7, address),
			city = COALESCE($8, city),
			postal_code = COALESCE($9, postal_code),
			language = COALESCE($10, language),
			timezone = COALESCE($11, timezone),
			updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`
	_, err := r.pool.Exec(ctx, query, req.UserID,
		req.Username, req.FirstName, req.LastName, req.Phone,
		req.DateOfBirth, req.Address, req.City, req.PostalCode,
		req.Language, req.Timezone,
	)
	if err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}
	return r.GetUserByID(ctx, req.UserID)
}

func (r *UserRepository) SoftDeleteUser(ctx context.Context, userID int64) error {
	query := `UPDATE users SET status = 'deleted', deleted_at = NOW(), updated_at = NOW() WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, userID)
	return err
}

func (r *UserRepository) GetPreferences(ctx context.Context, userID int64) (*domain.UserPreferences, error) {
	query := `
		SELECT user_id, language, timezone, currency_display, marketing_emails,
		       sms_notifications, push_notifications, reality_check,
		       reality_check_interval_minutes, auto_play, sound_preference, updated_at
		FROM user_preferences WHERE user_id = $1
	`
	pref := &domain.UserPreferences{}
	err := r.pool.QueryRow(ctx, query, userID).Scan(
		&pref.UserID, &pref.Language, &pref.Timezone, &pref.CurrencyDisplay,
		&pref.MarketingEmails, &pref.SMSNotifications, &pref.PushNotifications,
		&pref.RealityCheck, &pref.RealityCheckIntervalMinutes,
		&pref.AutoPlay, &pref.SoundPreference, &pref.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return &domain.UserPreferences{
			UserID:   userID,
			Language: "en",
			Timezone: "UTC",
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get preferences: %w", err)
	}
	return pref, nil
}

func (r *UserRepository) UpsertPreferences(ctx context.Context, pref *domain.UserPreferences) error {
	query := `
		INSERT INTO user_preferences (user_id, language, timezone, currency_display,
			marketing_emails, sms_notifications, push_notifications, reality_check,
			reality_check_interval_minutes, auto_play, sound_preference, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			language = COALESCE(NULLIF($2, ''), user_preferences.language),
			timezone = COALESCE(NULLIF($3, ''), user_preferences.timezone),
			currency_display = COALESCE(NULLIF($4, ''), user_preferences.currency_display),
			marketing_emails = $5,
			sms_notifications = $6,
			push_notifications = $7,
			reality_check = $8,
			reality_check_interval_minutes = $9,
			auto_play = $10,
			sound_preference = COALESCE(NULLIF($11, ''), user_preferences.sound_preference),
			updated_at = NOW()
	`
	_, err := r.pool.Exec(ctx, query, pref.UserID,
		pref.Language, pref.Timezone, pref.CurrencyDisplay,
		pref.MarketingEmails, pref.SMSNotifications, pref.PushNotifications,
		pref.RealityCheck, pref.RealityCheckIntervalMinutes,
		pref.AutoPlay, pref.SoundPreference,
	)
	return err
}

func (r *UserRepository) GetLimits(ctx context.Context, userID int64) (*domain.UserLimits, error) {
	query := `
		SELECT user_id, daily_deposit_limit, weekly_deposit_limit, monthly_deposit_limit,
		       daily_bet_limit, weekly_bet_limit, monthly_bet_limit,
		       daily_loss_limit, weekly_loss_limit, monthly_loss_limit,
		       session_time_limit_minutes, session_time_limit_active,
		       self_exclusion, self_exclusion_until, updated_at
		FROM user_limits WHERE user_id = $1
	`
	limits := &domain.UserLimits{}
	var sessionMinutes int
	var sessionActive bool
	err := r.pool.QueryRow(ctx, query, userID).Scan(
		&limits.UserID,
		&limits.DailyDepositLimit, &limits.WeeklyDepositLimit, &limits.MonthlyDepositLimit,
		&limits.DailyBetLimit, &limits.WeeklyBetLimit, &limits.MonthlyBetLimit,
		&limits.DailyLossLimit, &limits.WeeklyLossLimit, &limits.MonthlyLossLimit,
		&sessionMinutes, &sessionActive,
		&limits.SelfExclusion, &limits.SelfExclusionUntil, &limits.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return &domain.UserLimits{UserID: userID}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get limits: %w", err)
	}
	limits.SessionTimeLimit = &domain.TimeLimit{
		Minutes:  sessionMinutes,
		IsActive: sessionActive,
	}
	return limits, nil
}

func (r *UserRepository) SetLimits(ctx context.Context, userID int64, req *domain.SetLimitsRequest) error {
	query := `
		INSERT INTO user_limits (user_id, daily_deposit_limit, weekly_deposit_limit, monthly_deposit_limit,
			daily_bet_limit, weekly_bet_limit, monthly_bet_limit,
			daily_loss_limit, weekly_loss_limit, monthly_loss_limit,
			session_time_limit_minutes, session_time_limit_active,
			self_exclusion, self_exclusion_until, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			daily_deposit_limit = COALESCE($2, user_limits.daily_deposit_limit),
			weekly_deposit_limit = COALESCE($3, user_limits.weekly_deposit_limit),
			monthly_deposit_limit = COALESCE($4, user_limits.monthly_deposit_limit),
			daily_bet_limit = COALESCE($5, user_limits.daily_bet_limit),
			weekly_bet_limit = COALESCE($6, user_limits.weekly_bet_limit),
			monthly_bet_limit = COALESCE($7, user_limits.monthly_bet_limit),
			daily_loss_limit = COALESCE($8, user_limits.daily_loss_limit),
			weekly_loss_limit = COALESCE($9, user_limits.weekly_loss_limit),
			monthly_loss_limit = COALESCE($10, user_limits.monthly_loss_limit),
			session_time_limit_minutes = COALESCE($11, user_limits.session_time_limit_minutes),
			session_time_limit_active = COALESCE($12, user_limits.session_time_limit_active),
			self_exclusion = COALESCE($13, user_limits.self_exclusion),
			self_exclusion_until = COALESCE($14, user_limits.self_exclusion_until),
			updated_at = NOW()
	`
	var sessionMinutes *int
	var sessionActive *bool
	if req.SessionTimeMinutes != nil {
		sessionMinutes = req.SessionTimeMinutes
		v := true
		sessionActive = &v
	}

	_, err := r.pool.Exec(ctx, query, userID,
		req.DailyDepositLimit, req.WeeklyDepositLimit, req.MonthlyDepositLimit,
		req.DailyBetLimit, req.WeeklyBetLimit, req.MonthlyBetLimit,
		req.DailyLossLimit, req.WeeklyLossLimit, req.MonthlyLossLimit,
		sessionMinutes, sessionActive,
		req.SelfExclusion, req.SelfExclusionUntil,
	)
	return err
}

func (r *UserRepository) GetActivity(ctx context.Context, userID int64, limit, offset int) ([]map[string]interface{}, int, error) {
	countQuery := `SELECT COUNT(*) FROM audit_log WHERE record_id = $1 AND table_name = 'users'`
	var total int
	r.pool.QueryRow(ctx, countQuery, userID).Scan(&total)

	query := `
		SELECT id, action, old_data, new_data, user_id, created_at
		FROM audit_log WHERE record_id = $1 AND table_name = 'users'
		ORDER BY created_at DESC LIMIT $2 OFFSET $3
	`
	rows, err := r.pool.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var activities []map[string]interface{}
	for rows.Next() {
		var id int64
		var action, oldData, newData string
		var logUserID *int64
		var createdAt time.Time
		rows.Scan(&id, &action, &oldData, &newData, &logUserID, &createdAt)
		activities = append(activities, map[string]interface{}{
			"id":         id,
			"action":     action,
			"created_at": createdAt,
		})
	}
	return activities, total, nil
}
