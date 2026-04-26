package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/opus-casino/auth/internal/domain"
)

// AuthRepository handles user and session persistence
type AuthRepository struct {
	pool  *pgxpool.Pool
	redis *redis.Client
}

// NewAuthRepository creates a new auth repository
func NewAuthRepository(pool *pgxpool.Pool, redis *redis.Client) *AuthRepository {
	return &AuthRepository{pool: pool, redis: redis}
}

// CreateUser creates a new user
// NOTE: Real DB has hybrid schema (TypeORM camelCase + Go snake_case columns).
// We dual-write to both column sets for compatibility.
func (r *AuthRepository) CreateUser(ctx context.Context, user *domain.User) error {
	query := `
		INSERT INTO users (
			uuid, email, username, phone,
			"passwordHash", password_hash,
			status, country_code, currency_code,
			two_fa_enabled, "twoFactorEnabled",
			email_verified, "isEmailVerified",
			phone_verified, "isPhoneVerified",
			metadata
		)
		VALUES (
			$1, $2, $3, $4,
			$5, $5,
			$6, $7, $8,
			$9, $9,
			$10, $10,
			$11, $11,
			'{}'
		)
		RETURNING id::text, "createdAt", "updatedAt"
	`

	return r.pool.QueryRow(ctx, query,
		user.UUID, user.Email, user.Username, user.Phone,
		user.PasswordHash,
		user.Status, user.CountryCode, user.CurrencyCode,
		user.TwoFAEnabled, user.EmailVerified, user.PhoneVerified,
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
}

// GetUserByEmail retrieves a user by email
// NOTE: Uses COALESCE to read from either camelCase or snake_case columns.
func (r *AuthRepository) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `
		SELECT
			id::text,
			COALESCE(uuid::text, id::text),
			email,
			COALESCE(phone, ''),
			COALESCE(password_hash, "passwordHash"),
			COALESCE(username, ''),
			COALESCE(status, 'active'),
			COALESCE(kyc_level, 0),
			COALESCE(country_code, 'RU'),
			COALESCE(currency_code, currency, 'USD'),
			COALESCE(two_fa_enabled, "twoFactorEnabled", false),
			COALESCE(two_fa_secret, "twoFactorSecret"),
			COALESCE(email_verified, "isEmailVerified", false),
			COALESCE(phone_verified, "isPhoneVerified", false),
			COALESCE(created_at, "createdAt"),
			COALESCE(updated_at, "updatedAt"),
			last_login_at,
			COALESCE(metadata::text, '{}')
		FROM users
		WHERE email = $1 AND (deleted_at IS NULL AND "isBanned" = false)
	`

	user := &domain.User{}
	var phone string
	err := r.pool.QueryRow(ctx, query, email).Scan(
		&user.ID, &user.UUID, &user.Email, &phone,
		&user.PasswordHash, &user.Username,
		&user.Status, &user.KYCLevel,
		&user.CountryCode, &user.CurrencyCode,
		&user.TwoFAEnabled, &user.TwoFASecret,
		&user.EmailVerified, &user.PhoneVerified,
		&user.CreatedAt, &user.UpdatedAt, &user.LastLoginAt, &user.Metadata,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}
	if phone != "" {
		user.Phone = &phone
	}

	return user, nil
}

// GetUserByID retrieves a user by ID
// NOTE: Uses COALESCE to read from either camelCase or snake_case columns.
func (r *AuthRepository) GetUserByID(ctx context.Context, id string) (*domain.User, error) {
	query := `
		SELECT
			id::text,
			COALESCE(uuid::text, id::text),
			email,
			COALESCE(phone, ''),
			COALESCE(password_hash, "passwordHash"),
			COALESCE(username, ''),
			COALESCE(status, 'active'),
			COALESCE(kyc_level, 0),
			COALESCE(country_code, 'RU'),
			COALESCE(currency_code, currency, 'USD'),
			COALESCE(two_fa_enabled, "twoFactorEnabled", false),
			COALESCE(two_fa_secret, "twoFactorSecret"),
			COALESCE(email_verified, "isEmailVerified", false),
			COALESCE(phone_verified, "isPhoneVerified", false),
			COALESCE(created_at, "createdAt"),
			COALESCE(updated_at, "updatedAt"),
			last_login_at,
			COALESCE(metadata::text, '{}')
		FROM users
		WHERE id = $1::uuid AND (deleted_at IS NULL AND "isBanned" = false)
	`

	user := &domain.User{}
	var phone string
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&user.ID, &user.UUID, &user.Email, &phone,
		&user.PasswordHash, &user.Username,
		&user.Status, &user.KYCLevel,
		&user.CountryCode, &user.CurrencyCode,
		&user.TwoFAEnabled, &user.TwoFASecret,
		&user.EmailVerified, &user.PhoneVerified,
		&user.CreatedAt, &user.UpdatedAt, &user.LastLoginAt, &user.Metadata,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user by id: %w", err)
	}
	if phone != "" {
		user.Phone = &phone
	}

	return user, nil
}

// UpdateUser updates user fields (dual-write to both column sets)
func (r *AuthRepository) UpdateUser(ctx context.Context, user *domain.User) error {
	query := `
		UPDATE users
		SET two_fa_enabled = $2, "twoFactorEnabled" = $2,
		    two_fa_secret = $3, "twoFactorSecret" = $3,
		    updated_at = NOW(), "updatedAt" = NOW()
		WHERE id = $1::uuid
	`

	_, err := r.pool.Exec(ctx, query, user.ID, user.TwoFAEnabled, user.TwoFASecret)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	return nil
}

// UpdateLastLogin updates the user's last login timestamp
func (r *AuthRepository) UpdateLastLogin(ctx context.Context, userID string) error {
	query := `UPDATE users SET last_login_at = NOW(), "updatedAt" = NOW(), updated_at = NOW() WHERE id = $1::uuid`
	_, err := r.pool.Exec(ctx, query, userID)
	return err
}

// UpdatePassword updates the user's password hash (dual-write both columns)
func (r *AuthRepository) UpdatePassword(ctx context.Context, userID string, passwordHash string) error {
	query := `UPDATE users SET password_hash = $2, "passwordHash" = $2, updated_at = NOW(), "updatedAt" = NOW() WHERE id = $1::uuid`
	_, err := r.pool.Exec(ctx, query, userID, passwordHash)
	return err
}

// CreateSession creates a new session in Redis
func (r *AuthRepository) CreateSession(ctx context.Context, session *domain.Session) error {
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	ttl := time.Until(session.ExpiresAt)
	key := fmt.Sprintf("session:%s", session.ID)

	if err := r.redis.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("failed to create session in Redis: %w", err)
	}

	// Also store in user's session set for management
	userKey := fmt.Sprintf("user_sessions:%s", session.UserID)
	r.redis.SAdd(ctx, userKey, session.ID)
	r.redis.Expire(ctx, userKey, ttl)

	return nil
}

// GetSession retrieves a session from Redis
func (r *AuthRepository) GetSession(ctx context.Context, sessionID string) (*domain.Session, error) {
	key := fmt.Sprintf("session:%s", sessionID)
	data, err := r.redis.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	var session domain.Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	return &session, nil
}

// DeleteSession deletes a session from Redis
func (r *AuthRepository) DeleteSession(ctx context.Context, sessionID string, userID string) error {
	key := fmt.Sprintf("session:%s", sessionID)
	r.redis.Del(ctx, key)

	// Remove from user's session set
	userKey := fmt.Sprintf("user_sessions:%s", userID)
	r.redis.SRem(ctx, userKey, sessionID)

	return nil
}

// GetUserSessions retrieves all active session IDs for a user
func (r *AuthRepository) GetUserSessions(ctx context.Context, userID string) ([]string, error) {
	key := fmt.Sprintf("user_sessions:%s", userID)
	return r.redis.SMembers(ctx, key).Result()
}

// DeleteAllUserSessions deletes all sessions for a user
func (r *AuthRepository) DeleteAllUserSessions(ctx context.Context, userID string) error {
	key := fmt.Sprintf("user_sessions:%s", userID)
	sessionIDs, err := r.redis.SMembers(ctx, key).Result()
	if err != nil {
		return err
	}

	for _, sid := range sessionIDs {
		r.redis.Del(ctx, fmt.Sprintf("session:%s", sid))
	}
	r.redis.Del(ctx, key)

	return nil
}

// StoreRefreshToken stores a refresh token in Redis
func (r *AuthRepository) StoreRefreshToken(ctx context.Context, token string, userID string, sessionID string, ttl time.Duration) error {
	key := fmt.Sprintf("refresh_token:%s", token)
	data := fmt.Sprintf("%s:%s", userID, sessionID)
	return r.redis.Set(ctx, key, data, ttl).Err()
}

// GetRefreshToken retrieves a refresh token from Redis
func (r *AuthRepository) GetRefreshToken(ctx context.Context, token string) (userID string, sessionID string, err error) {
	key := fmt.Sprintf("refresh_token:%s", token)
	data, err := r.redis.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return "", "", nil
		}
		return "", "", err
	}

	fmt.Sscanf(data, "%s:%s", &userID, &sessionID)
	return userID, sessionID, nil
}

// DeleteRefreshToken deletes a refresh token from Redis
func (r *AuthRepository) DeleteRefreshToken(ctx context.Context, token string) error {
	key := fmt.Sprintf("refresh_token:%s", token)
	return r.redis.Del(ctx, key).Err()
}

// TrackLoginAttempt tracks failed login attempts
func (r *AuthRepository) TrackLoginAttempt(ctx context.Context, email, ip string) (attempts int, locked bool, err error) {
	key := fmt.Sprintf("login_attempts:%s:%s", email, ip)

	val, err := r.redis.Incr(ctx, key).Result()
	if err != nil {
		return 0, false, err
	}

	// Set expiry on first attempt
	if val == 1 {
		r.redis.Expire(ctx, key, 15*time.Minute)
	}

	// Lock after 10 failed attempts
	if val >= 10 {
		lockKey := fmt.Sprintf("login_lock:%s", email)
		r.redis.Set(ctx, lockKey, "1", 30*time.Minute)
		return int(val), true, nil
	}

	return int(val), false, nil
}

// IsAccountLocked checks if an account is locked
func (r *AuthRepository) IsAccountLocked(ctx context.Context, email string) (bool, error) {
	key := fmt.Sprintf("login_lock:%s", email)
	exists, err := r.redis.Exists(ctx, key).Result()
	return exists > 0, err
}

// ClearLoginAttempts clears failed login attempts
func (r *AuthRepository) ClearLoginAttempts(ctx context.Context, email, ip string) error {
	key := fmt.Sprintf("login_attempts:%s:%s", email, ip)
	return r.redis.Del(ctx, key).Err()
}

// StoreTempToken stores a temporary token for 2FA flow
func (r *AuthRepository) StoreTempToken(ctx context.Context, token string, userID string, ttl time.Duration) error {
	key := fmt.Sprintf("temp_token:%s", token)
	return r.redis.Set(ctx, key, userID, ttl).Err()
}

// GetTempToken retrieves a temporary token for 2FA flow
func (r *AuthRepository) GetTempToken(ctx context.Context, token string) (string, error) {
	key := fmt.Sprintf("temp_token:%s", token)
	val, err := r.redis.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	return val, err
}

// DeleteTempToken deletes a temporary token
func (r *AuthRepository) DeleteTempToken(ctx context.Context, token string) error {
	key := fmt.Sprintf("temp_token:%s", token)
	return r.redis.Del(ctx, key).Err()
}
