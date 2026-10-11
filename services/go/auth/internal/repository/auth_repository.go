package repository

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/opus-casino/auth/internal/crypto"
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

// CreateUser creates a new user in the canonical users schema.
func (r *AuthRepository) CreateUser(ctx context.Context, user *domain.User) error {
	metadata, err := json.Marshal(map[string]string{
		"username": user.Username,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal user metadata: %w", err)
	}

	query := `
		INSERT INTO users (
			uuid, email, phone,
			password_hash,
			status, country_code, currency_code,
			two_fa_enabled,
			email_verified,
			phone_verified,
			metadata
		)
		VALUES (
			$1, $2, $3,
			$4,
			$5, $6, $7,
			$8,
			$9,
			$10,
			$11::jsonb
		)
		RETURNING id::text, created_at, updated_at
	`

	err = r.pool.QueryRow(ctx, query,
		user.UUID, user.Email, user.Phone,
		user.PasswordHash,
		user.Status, user.CountryCode, user.CurrencyCode,
		user.TwoFAEnabled, user.EmailVerified, user.PhoneVerified,
		string(metadata),
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrUserAlreadyExists
		}
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// GetUserByEmail retrieves a user by email from the canonical users schema.
func (r *AuthRepository) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `
		SELECT
			id::text,
			uuid::text,
			email,
			COALESCE(phone, ''),
			password_hash,
			COALESCE(metadata->>'username', ''),
			status,
			kyc_level,
			country_code,
			currency_code,
			two_fa_enabled,
			two_fa_secret,
			email_verified,
			phone_verified,
			created_at,
			updated_at,
			last_login_at,
			COALESCE(metadata::text, '{}')
		FROM users
		WHERE email = $1 AND deleted_at IS NULL
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

// GetUserByIdentifier retrieves a user by email or username (stored in metadata->username), case-insensitive.
func (r *AuthRepository) GetUserByIdentifier(ctx context.Context, identifier string) (*domain.User, error) {
	identifier = strings.TrimSpace(strings.ToLower(identifier))
	if identifier == "" {
		return nil, nil
	}

	// Prefer direct email lookup for exact matches.
	if _, err := mail.ParseAddress(identifier); err == nil {
		return r.GetUserByEmail(ctx, identifier)
	}

	query := `
		SELECT
			id::text,
			uuid::text,
			email,
			COALESCE(phone, ''),
			password_hash,
			COALESCE(metadata->>'username', ''),
			status,
			kyc_level,
			country_code,
			currency_code,
			two_fa_enabled,
			two_fa_secret,
			email_verified,
			phone_verified,
			created_at,
			updated_at,
			last_login_at,
			COALESCE(metadata::text, '{}')
		FROM users
		WHERE (
			LOWER(COALESCE(metadata->>'username', '')) = $1
			OR LOWER(email) = $1
			OR LOWER(split_part(email, '@', 1)) = $1
		) AND deleted_at IS NULL
		LIMIT 1
	`

	user := &domain.User{}
	var phone string
	err := r.pool.QueryRow(ctx, query, identifier).Scan(
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
		return nil, fmt.Errorf("failed to get user by identifier: %w", err)
	}
	if phone != "" {
		user.Phone = &phone
	}

	return user, nil
}

// GetUserByGoogleSub retrieves a user linked with Google account.
func (r *AuthRepository) GetUserByGoogleSub(ctx context.Context, googleSub string) (*domain.User, error) {
	query := `
		SELECT
			id::text,
			uuid::text,
			email,
			COALESCE(phone, ''),
			password_hash,
			COALESCE(metadata->>'username', ''),
			status,
			kyc_level,
			country_code,
			currency_code,
			two_fa_enabled,
			two_fa_secret,
			email_verified,
			phone_verified,
			created_at,
			updated_at,
			last_login_at,
			COALESCE(metadata::text, '{}')
		FROM users
		WHERE COALESCE(metadata->>'google_sub', '') = $1 AND deleted_at IS NULL
		LIMIT 1
	`

	user := &domain.User{}
	var phone string
	err := r.pool.QueryRow(ctx, query, googleSub).Scan(
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
		return nil, fmt.Errorf("failed to get user by google sub: %w", err)
	}
	if phone != "" {
		user.Phone = &phone
	}

	return user, nil
}

// LinkGoogleSub links Google account data to existing user metadata.
func (r *AuthRepository) LinkGoogleSub(ctx context.Context, userID, googleSub string, emailVerified bool) error {
	query := `
		UPDATE users
		SET metadata = jsonb_set(
				jsonb_set(COALESCE(metadata, '{}'::jsonb), '{google_sub}', to_jsonb($2::text), true),
				'{auth_provider}',
				to_jsonb('google'::text),
				true
			),
			email_verified = $3,
			updated_at = NOW()
		WHERE id = $1::bigint
	`
	_, err := r.pool.Exec(ctx, query, userID, googleSub, emailVerified)
	if err != nil {
		return fmt.Errorf("failed to link google sub: %w", err)
	}
	return nil
}

func generateGoogleFallbackPassword() string {
	buf := make([]byte, 24)
	_, _ = rand.Read(buf)
	return fmt.Sprintf("google-%x", buf)
}

// CreateUserFromGoogle creates a user authenticated by Google OAuth.
func (r *AuthRepository) CreateUserFromGoogle(ctx context.Context, email, googleSub, username string) (*domain.User, error) {
	passwordHash, err := crypto.HashPassword(generateGoogleFallbackPassword())
	if err != nil {
		return nil, fmt.Errorf("failed to hash google fallback password: %w", err)
	}

	user := &domain.User{
		UUID:          uuid.NewString(),
		Email:         strings.ToLower(strings.TrimSpace(email)),
		Username:      strings.TrimSpace(username),
		PasswordHash:  passwordHash,
		Status:        domain.UserStatusActive,
		CountryCode:   "US",
		CurrencyCode:  "USD",
		TwoFAEnabled:  false,
		EmailVerified: true,
		PhoneVerified: false,
	}

	metadata, err := json.Marshal(map[string]string{
		"username":      user.Username,
		"google_sub":    googleSub,
		"auth_provider": "google",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal google user metadata: %w", err)
	}

	query := `
		INSERT INTO users (
			uuid, email, password_hash, status, country_code, currency_code,
			two_fa_enabled, email_verified, phone_verified, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::jsonb)
		RETURNING id::text, created_at, updated_at
	`

	err = r.pool.QueryRow(ctx, query,
		user.UUID, user.Email, user.PasswordHash, user.Status, user.CountryCode, user.CurrencyCode,
		user.TwoFAEnabled, user.EmailVerified, user.PhoneVerified, string(metadata),
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create google user: %w", err)
	}

	return user, nil
}

// GetUserByID retrieves a user by ID from the canonical users schema.
func (r *AuthRepository) GetUserByID(ctx context.Context, id string) (*domain.User, error) {
	query := `
		SELECT
			id::text,
			uuid::text,
			email,
			COALESCE(phone, ''),
			password_hash,
			COALESCE(metadata->>'username', ''),
			status,
			kyc_level,
			country_code,
			currency_code,
			two_fa_enabled,
			two_fa_secret,
			email_verified,
			phone_verified,
			created_at,
			updated_at,
			last_login_at,
			COALESCE(metadata::text, '{}')
		FROM users
		WHERE id = $1::bigint AND deleted_at IS NULL
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

// UpdateUser updates mutable user fields.
func (r *AuthRepository) UpdateUser(ctx context.Context, user *domain.User) error {
	query := `
		UPDATE users
		SET two_fa_enabled = $2,
		    two_fa_secret = $3,
		    updated_at = NOW()
		WHERE id = $1::bigint
	`

	_, err := r.pool.Exec(ctx, query, user.ID, user.TwoFAEnabled, user.TwoFASecret)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	return nil
}

// UpdateLastLogin updates the user's last login timestamp
func (r *AuthRepository) UpdateLastLogin(ctx context.Context, userID string) error {
	query := `UPDATE users SET last_login_at = NOW(), updated_at = NOW() WHERE id = $1::bigint`
	_, err := r.pool.Exec(ctx, query, userID)
	return err
}

// UpdatePassword updates the user's password hash.
func (r *AuthRepository) UpdatePassword(ctx context.Context, userID string, passwordHash string) error {
	query := `UPDATE users SET password_hash = $2, updated_at = NOW() WHERE id = $1::bigint`
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
		return fmt.Errorf("%w: failed to create session in redis", domain.ErrDependencyUnavailable)
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
		return nil, fmt.Errorf("%w: failed to get session from redis", domain.ErrDependencyUnavailable)
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
		if err := r.redis.Del(ctx, fmt.Sprintf("session:%s", sid)).Err(); err != nil {
			return fmt.Errorf("failed to delete session %s: %w", sid, err)
		}
	}

	if err := r.redis.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete session index: %w", err)
	}

	return nil
}

// Refresh token rotation with reuse detection.
//
// Active tokens live under refresh_token:<token>. When a token is rotated it is
// moved to refresh_rotated:<token>, keeping its family id and the remaining
// lifetime. If a token is later presented again while sitting in the rotated
// set, it has been stolen and replayed: the whole family is revoked and the
// caller must fail the refresh (see RotateRefreshToken).
const refreshTokenPrefix = "refresh_token:"
const rotatedRefreshTokenPrefix = "refresh_rotated:"

// StoreRefreshToken stores a refresh token in Redis
func (r *AuthRepository) StoreRefreshToken(ctx context.Context, token string, userID string, sessionID string, ttl time.Duration) error {
	key := fmt.Sprintf("%s%s", refreshTokenPrefix, token)
	data := fmt.Sprintf("%s:%s", userID, sessionID)
	if err := r.redis.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("%w: failed to store refresh token in redis", domain.ErrDependencyUnavailable)
	}
	return nil
}

// RotateRefreshToken consumes a refresh token and issues the next one in the
// same family.
//
// Returns (userID, sessionID, newToken, err). When err is
// domain.ErrRefreshTokenReuse the token had already been rotated — the whole
// family and all user sessions have been revoked and the caller must reject
// the request.
func (r *AuthRepository) RotateRefreshToken(
	ctx context.Context,
	currentToken string,
	newToken string,
	ttl time.Duration,
) (userID string, sessionID string, err error) {
	key := fmt.Sprintf("%s%s", refreshTokenPrefix, currentToken)
	rotatedKey := fmt.Sprintf("%s%s", rotatedRefreshTokenPrefix, currentToken)

	// Atomically move the token to the rotated set. GETDEL-style semantics:
	// only one caller can win the rotation.
	pipeline := r.redis.TxPipeline()
	moved := pipeline.GetDel(ctx, key)
	if _, execErr := pipeline.Exec(ctx); execErr != nil && execErr != redis.Nil {
		return "", "", fmt.Errorf("%w: failed to read refresh token in redis", domain.ErrDependencyUnavailable)
	}

	value, err := moved.Val(), moved.Err()
	switch {
	case err == redis.Nil:
		// Not active: either unknown or already rotated. Distinguish reuse.
		if r.redis.Exists(ctx, rotatedKey).Val() > 0 {
			return "", "", domain.ErrRefreshTokenReuse
		}
		return "", "", domain.ErrInvalidRefreshToken
	case err != nil:
		return "", "", fmt.Errorf("%w: failed to read refresh token in redis", domain.ErrDependencyUnavailable)
	}

	parts := strings.SplitN(value, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", domain.ErrInvalidRefreshToken
	}
	userID, sessionID = parts[0], parts[1]

	// Remember the consumed token for reuse detection, bounded by the new
	// token's lifetime so the rotated set cannot grow without limit.
	ttlLeft := ttl
	if remaining, err := r.redis.TTL(ctx, key).Result(); err == nil && remaining > 0 {
		ttlLeft = remaining
	}
	if err := r.redis.Set(ctx, rotatedKey, userID+":"+sessionID, ttlLeft).Err(); err != nil {
		return "", "", fmt.Errorf("%w: failed to record rotated refresh token in redis", domain.ErrDependencyUnavailable)
	}

	if err := r.redis.Set(ctx, fmt.Sprintf("%s%s", refreshTokenPrefix, newToken),
		userID+":"+sessionID, ttl).Err(); err != nil {
		return "", "", fmt.Errorf("%w: failed to store refresh token in redis", domain.ErrDependencyUnavailable)
	}

	return userID, sessionID, nil
}

// RevokeRefreshTokenFamily removes every active refresh token of a session,
// which is what happens when token reuse is detected.
func (r *AuthRepository) RevokeRefreshTokenFamily(ctx context.Context, userID, sessionID string) error {
	pattern := fmt.Sprintf("%s*", refreshTokenPrefix)
	iter := r.redis.Scan(ctx, 0, pattern, 100).Iterator()

	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return fmt.Errorf("%w: failed to scan refresh tokens in redis", domain.ErrDependencyUnavailable)
	}

	for _, key := range keys {
		value, err := r.redis.Get(ctx, key).Result()
		if err != nil {
			continue
		}
		if parts := strings.SplitN(value, ":", 2); len(parts) == 2 &&
			parts[0] == userID && (sessionID == "" || parts[1] == sessionID) {
			if err := r.redis.Del(ctx, key).Err(); err != nil {
				return fmt.Errorf("%w: failed to revoke refresh token in redis", domain.ErrDependencyUnavailable)
			}
		}
	}

	return nil
}

// GetRefreshToken retrieves a refresh token from Redis
func (r *AuthRepository) GetRefreshToken(ctx context.Context, token string) (userID string, sessionID string, err error) {
	key := fmt.Sprintf("refresh_token:%s", token)
	data, err := r.redis.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return "", "", nil
		}
		return "", "", fmt.Errorf("%w: failed to get refresh token from redis", domain.ErrDependencyUnavailable)
	}

	parts := strings.SplitN(data, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid refresh token payload")
	}

	return parts[0], parts[1], nil
}

// DeleteRefreshToken deletes a refresh token from Redis
func (r *AuthRepository) DeleteRefreshToken(ctx context.Context, token string) error {
	key := fmt.Sprintf("refresh_token:%s", token)
	if err := r.redis.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("%w: failed to delete refresh token from redis", domain.ErrDependencyUnavailable)
	}
	return nil
}

// Login attempt limits. Failures are counted per identifier (email/username)
// and per source IP independently, so neither a single account hammered from
// many IPs nor one IP spraying many accounts can slip under the limit.
const (
	loginAttemptWindow    = 15 * time.Minute
	loginLockDuration     = 30 * time.Minute
	maxAttemptsPerAccount = 5
	maxAttemptsPerIP      = 10
)

// TrackLoginAttempt tracks failed login attempts.
//
// Both counters are incremented: one keyed by the identifier, one keyed by the
// IP. The account is locked as soon as either threshold is reached.
func (r *AuthRepository) TrackLoginAttempt(ctx context.Context, email, ip string) (attempts int, locked bool, err error) {
	accountKey := loginAttemptsAccountKey(email)
	ipKey := loginAttemptsIPKey(ip)

	accountCount, err := r.incrWithWindow(ctx, accountKey)
	if err != nil {
		return 0, false, err
	}

	ipCount := int64(0)
	if ip != "" {
		ipCount, err = r.incrWithWindow(ctx, ipKey)
		if err != nil {
			return 0, false, err
		}
	}

	if accountCount >= maxAttemptsPerAccount || ipCount >= maxAttemptsPerIP {
		lockKey := fmt.Sprintf("login_lock:%s", strings.ToLower(email))
		if err := r.redis.Set(ctx, lockKey, "1", loginLockDuration).Err(); err != nil {
			return int(accountCount), false, fmt.Errorf(
				"%w: failed to lock account in redis", domain.ErrDependencyUnavailable)
		}
		return int(accountCount), true, nil
	}

	return int(accountCount), false, nil
}

// incrWithWindow increments a counter and applies the TTL on first use.
func (r *AuthRepository) incrWithWindow(ctx context.Context, key string) (int64, error) {
	val, err := r.redis.Incr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("%w: failed to increment login attempts in redis", domain.ErrDependencyUnavailable)
	}
	if val == 1 {
		if err := r.redis.Expire(ctx, key, loginAttemptWindow).Err(); err != nil {
			return 0, fmt.Errorf("%w: failed to set login attempt window in redis", domain.ErrDependencyUnavailable)
		}
	}
	return val, nil
}

func loginAttemptsAccountKey(email string) string {
	return fmt.Sprintf("login_attempts:account:%s", strings.ToLower(strings.TrimSpace(email)))
}

func loginAttemptsIPKey(ip string) string {
	return fmt.Sprintf("login_attempts:ip:%s", strings.TrimSpace(ip))
}

// IsAccountLocked checks if an account is locked
func (r *AuthRepository) IsAccountLocked(ctx context.Context, email string) (bool, error) {
	key := fmt.Sprintf("login_lock:%s", strings.ToLower(strings.TrimSpace(email)))
	exists, err := r.redis.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("%w: failed to check account lock in redis", domain.ErrDependencyUnavailable)
	}
	return exists > 0, nil
}

// ClearLoginAttempts clears failed login attempts for an account and IP.
//
// Called after a successful authentication, so it also lifts the lockout:
// a successful login proves the credentials were not guessed. The IP counter
// is cleared as well, otherwise one legitimate login would keep an unrelated
// user on the same NAT locked out.
func (r *AuthRepository) ClearLoginAttempts(ctx context.Context, email, ip string) error {
	keys := []string{loginAttemptsAccountKey(email)}
	if ip != "" {
		keys = append(keys, loginAttemptsIPKey(ip))
	}
	keys = append(keys, fmt.Sprintf("login_lock:%s", strings.ToLower(strings.TrimSpace(email))))

	if err := r.redis.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("%w: failed to clear login attempts in redis", domain.ErrDependencyUnavailable)
	}
	return nil
}

// StoreTempToken stores a temporary token for 2FA flow
func (r *AuthRepository) StoreTempToken(ctx context.Context, token string, userID string, ttl time.Duration) error {
	key := fmt.Sprintf("temp_token:%s", token)
	if err := r.redis.Set(ctx, key, userID, ttl).Err(); err != nil {
		return fmt.Errorf("%w: failed to store temp token in redis", domain.ErrDependencyUnavailable)
	}
	return nil
}

// GetTempToken retrieves a temporary token for 2FA flow
func (r *AuthRepository) GetTempToken(ctx context.Context, token string) (string, error) {
	key := fmt.Sprintf("temp_token:%s", token)
	val, err := r.redis.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("%w: failed to get temp token from redis", domain.ErrDependencyUnavailable)
	}
	return val, nil
}

// DeleteTempToken deletes a temporary token
func (r *AuthRepository) DeleteTempToken(ctx context.Context, token string) error {
	key := fmt.Sprintf("temp_token:%s", token)
	if err := r.redis.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("%w: failed to delete temp token in redis", domain.ErrDependencyUnavailable)
	}
	return nil
}
