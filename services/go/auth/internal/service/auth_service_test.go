package service

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/opus-casino/auth/internal/crypto"
	"github.com/opus-casino/auth/internal/domain"
)

type mockAuthRepository struct {
	getUserByEmail    func(context.Context, string) (*domain.User, error)
	getUserByIdentifier func(context.Context, string) (*domain.User, error)
	getUserByID       func(context.Context, string) (*domain.User, error)
	getUserByGoogleSub func(context.Context, string) (*domain.User, error)
	createUser        func(context.Context, *domain.User) error
	createUserFromGoogle func(context.Context, string, string, string) (*domain.User, error)
	linkGoogleSub     func(context.Context, string, string, bool) error
	updateUser        func(context.Context, *domain.User) error
	updateLastLogin   func(context.Context, string) error
	updatePassword    func(context.Context, string, string) error
	createSession     func(context.Context, *domain.Session) error
	getSession        func(context.Context, string) (*domain.Session, error)
	deleteSession     func(context.Context, string, string) error
	getUserSessions   func(context.Context, string) ([]string, error)
	deleteAllSessions func(context.Context, string) error
	storeRefreshToken func(context.Context, string, string, string, time.Duration) error
	getRefreshToken   func(context.Context, string) (string, string, error)
	deleteRefresh     func(context.Context, string) error
	trackAttempt      func(context.Context, string, string) (int, bool, error)
	isLocked          func(context.Context, string) (bool, error)
	clearAttempts     func(context.Context, string, string) error
	storeTempToken    func(context.Context, string, string, time.Duration) error
	getTempToken      func(context.Context, string) (string, error)
	deleteTempToken   func(context.Context, string) error
}

func (m *mockAuthRepository) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	if m.getUserByEmail != nil {
		return m.getUserByEmail(ctx, email)
	}
	return nil, nil
}

func (m *mockAuthRepository) GetUserByIdentifier(ctx context.Context, identifier string) (*domain.User, error) {
	if m.getUserByIdentifier != nil {
		return m.getUserByIdentifier(ctx, identifier)
	}
	return m.GetUserByEmail(ctx, identifier)
}

func (m *mockAuthRepository) GetUserByID(ctx context.Context, id string) (*domain.User, error) {
	if m.getUserByID != nil {
		return m.getUserByID(ctx, id)
	}
	return nil, nil
}

func (m *mockAuthRepository) GetUserByGoogleSub(ctx context.Context, googleSub string) (*domain.User, error) {
	if m.getUserByGoogleSub != nil {
		return m.getUserByGoogleSub(ctx, googleSub)
	}
	return nil, nil
}

func (m *mockAuthRepository) CreateUser(ctx context.Context, user *domain.User) error {
	if m.createUser != nil {
		return m.createUser(ctx, user)
	}
	return nil
}

func (m *mockAuthRepository) CreateUserFromGoogle(ctx context.Context, email, googleSub, username string) (*domain.User, error) {
	if m.createUserFromGoogle != nil {
		return m.createUserFromGoogle(ctx, email, googleSub, username)
	}
	return &domain.User{ID: "google-user", Email: email, Username: username}, nil
}

func (m *mockAuthRepository) LinkGoogleSub(ctx context.Context, userID, googleSub string, emailVerified bool) error {
	if m.linkGoogleSub != nil {
		return m.linkGoogleSub(ctx, userID, googleSub, emailVerified)
	}
	return nil
}

func (m *mockAuthRepository) UpdateUser(ctx context.Context, user *domain.User) error {
	if m.updateUser != nil {
		return m.updateUser(ctx, user)
	}
	return nil
}

func (m *mockAuthRepository) UpdateLastLogin(ctx context.Context, userID string) error {
	if m.updateLastLogin != nil {
		return m.updateLastLogin(ctx, userID)
	}
	return nil
}

func (m *mockAuthRepository) UpdatePassword(ctx context.Context, userID string, passwordHash string) error {
	if m.updatePassword != nil {
		return m.updatePassword(ctx, userID, passwordHash)
	}
	return nil
}

func (m *mockAuthRepository) CreateSession(ctx context.Context, session *domain.Session) error {
	if m.createSession != nil {
		return m.createSession(ctx, session)
	}
	return nil
}

func (m *mockAuthRepository) GetSession(ctx context.Context, sessionID string) (*domain.Session, error) {
	if m.getSession != nil {
		return m.getSession(ctx, sessionID)
	}
	return nil, nil
}

func (m *mockAuthRepository) DeleteSession(ctx context.Context, sessionID string, userID string) error {
	if m.deleteSession != nil {
		return m.deleteSession(ctx, sessionID, userID)
	}
	return nil
}

func (m *mockAuthRepository) GetUserSessions(ctx context.Context, userID string) ([]string, error) {
	if m.getUserSessions != nil {
		return m.getUserSessions(ctx, userID)
	}
	return nil, nil
}

func (m *mockAuthRepository) DeleteAllUserSessions(ctx context.Context, userID string) error {
	if m.deleteAllSessions != nil {
		return m.deleteAllSessions(ctx, userID)
	}
	return nil
}

func (m *mockAuthRepository) StoreRefreshToken(ctx context.Context, token string, userID string, sessionID string, ttl time.Duration) error {
	if m.storeRefreshToken != nil {
		return m.storeRefreshToken(ctx, token, userID, sessionID, ttl)
	}
	return nil
}

func (m *mockAuthRepository) GetRefreshToken(ctx context.Context, token string) (string, string, error) {
	if m.getRefreshToken != nil {
		return m.getRefreshToken(ctx, token)
	}
	return "", "", nil
}

func (m *mockAuthRepository) DeleteRefreshToken(ctx context.Context, token string) error {
	if m.deleteRefresh != nil {
		return m.deleteRefresh(ctx, token)
	}
	return nil
}

func (m *mockAuthRepository) TrackLoginAttempt(ctx context.Context, email, ip string) (int, bool, error) {
	if m.trackAttempt != nil {
		return m.trackAttempt(ctx, email, ip)
	}
	return 1, false, nil
}

func (m *mockAuthRepository) IsAccountLocked(ctx context.Context, email string) (bool, error) {
	if m.isLocked != nil {
		return m.isLocked(ctx, email)
	}
	return false, nil
}

func (m *mockAuthRepository) ClearLoginAttempts(ctx context.Context, email, ip string) error {
	if m.clearAttempts != nil {
		return m.clearAttempts(ctx, email, ip)
	}
	return nil
}

func (m *mockAuthRepository) StoreTempToken(ctx context.Context, token string, userID string, ttl time.Duration) error {
	if m.storeTempToken != nil {
		return m.storeTempToken(ctx, token, userID, ttl)
	}
	return nil
}

func (m *mockAuthRepository) GetTempToken(ctx context.Context, token string) (string, error) {
	if m.getTempToken != nil {
		return m.getTempToken(ctx, token)
	}
	return "", nil
}

func (m *mockAuthRepository) DeleteTempToken(ctx context.Context, token string) error {
	if m.deleteTempToken != nil {
		return m.deleteTempToken(ctx, token)
	}
	return nil
}

func newTestService(repo *mockAuthRepository) *AuthService {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		panic(err)
	}
	// Reuse production constructor to avoid diverging behavior.
	jwtCfg, err := crypto.NewEd25519JWTConfigFromBase64(
		// base64.StdEncoding is used by config; mirror it here.
		cryptoMustB64(priv),
		cryptoMustB64(pub),
	)
	if err != nil {
		panic(err)
	}
	return NewAuthService(repo, jwtCfg, zap.NewNop())
}

func cryptoMustB64(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

func TestRegister_Success(t *testing.T) {
	repo := &mockAuthRepository{
		createUser: func(_ context.Context, user *domain.User) error {
			user.ID = "user-1"
			return nil
		},
	}
	svc := newTestService(repo)

	result, err := svc.Register(context.Background(), &domain.RegisterRequest{
		Email:        "test@example.com",
		Password:     "Password123",
		Username:     "tester",
		CountryCode:  "ru",
		CurrencyCode: "rub",
		DeviceID:     "web-device",
		IPAddress:    "127.0.0.1",
	})
	require.NoError(t, err)
	require.Equal(t, "user-1", result.UserID)
	require.NotNil(t, result.Tokens)
	require.NotEmpty(t, result.Tokens.AccessToken)
	require.NotEmpty(t, result.Tokens.RefreshToken)
}

func TestRegister_DuplicateEmail(t *testing.T) {
	repo := &mockAuthRepository{
		getUserByEmail: func(_ context.Context, _ string) (*domain.User, error) {
			return &domain.User{ID: "existing"}, nil
		},
	}
	svc := newTestService(repo)

	_, err := svc.Register(context.Background(), &domain.RegisterRequest{
		Email:        "dup@example.com",
		Password:     "Password123",
		Username:     "tester",
		CountryCode:  "RU",
		CurrencyCode: "RUB",
	})
	require.ErrorIs(t, err, domain.ErrUserAlreadyExists)
}

func TestLogin_Success(t *testing.T) {
	hash, err := crypto.HashPassword("Password123")
	require.NoError(t, err)

	repo := &mockAuthRepository{
		getUserByEmail: func(_ context.Context, _ string) (*domain.User, error) {
			return &domain.User{
				ID:           "user-1",
				Email:        "user@example.com",
				PasswordHash: hash,
				CountryCode:  "RU",
			}, nil
		},
	}
	svc := newTestService(repo)

	result, err := svc.Login(context.Background(), &domain.LoginRequest{
		Email:    "user@example.com",
		Password: "Password123",
		DeviceID: "web-device",
	})
	require.NoError(t, err)
	require.Equal(t, "user-1", result.UserID)
	require.NotEmpty(t, result.Tokens.AccessToken)
}

func TestLogin_InvalidCreds(t *testing.T) {
	repo := &mockAuthRepository{
		getUserByEmail: func(_ context.Context, _ string) (*domain.User, error) {
			return nil, nil
		},
	}
	svc := newTestService(repo)

	_, err := svc.Login(context.Background(), &domain.LoginRequest{
		Email:    "missing@example.com",
		Password: "Password123",
	})
	require.ErrorIs(t, err, domain.ErrInvalidCredentials)
}

func TestLogin_ByIdentifierUsername_Success(t *testing.T) {
	hash, err := crypto.HashPassword("Password123")
	require.NoError(t, err)

	repo := &mockAuthRepository{
		getUserByIdentifier: func(_ context.Context, identifier string) (*domain.User, error) {
			require.Equal(t, "nickname", identifier)
			return &domain.User{
				ID:           "user-2",
				Email:        "user2@example.com",
				PasswordHash: hash,
				CountryCode:  "RU",
			}, nil
		},
	}
	svc := newTestService(repo)

	result, err := svc.Login(context.Background(), &domain.LoginRequest{
		Identifier: "nickname",
		Password:   "Password123",
	})
	require.NoError(t, err)
	require.Equal(t, "user-2", result.UserID)
}

func TestMe_WithValidToken(t *testing.T) {
	repo := &mockAuthRepository{
		getSession: func(_ context.Context, sessionID string) (*domain.Session, error) {
			if sessionID == "session-1" {
				return &domain.Session{ID: "session-1", UserID: "user-1"}, nil
			}
			return nil, nil
		},
		getUserByID: func(_ context.Context, id string) (*domain.User, error) {
			if id == "user-1" {
				return &domain.User{ID: "user-1", Email: "user@example.com"}, nil
			}
			return nil, nil
		},
	}
	svc := newTestService(repo)

	token, err := svc.jwtConfig.GenerateAccessToken("user-1", "session-1", "web-device")
	require.NoError(t, err)

	claims, err := svc.ValidateToken(context.Background(), token)
	require.NoError(t, err)
	require.Equal(t, "user-1", claims.UserID)

	user, err := svc.GetCurrentUser(context.Background(), claims.UserID)
	require.NoError(t, err)
	require.Equal(t, "user-1", user.ID)
}

func TestRegister_DuplicateFromRepository(t *testing.T) {
	repo := &mockAuthRepository{
		createUser: func(_ context.Context, _ *domain.User) error {
			return domain.ErrUserAlreadyExists
		},
	}
	svc := newTestService(repo)

	_, err := svc.Register(context.Background(), &domain.RegisterRequest{
		Email:        "dup@example.com",
		Password:     "Password123",
		Username:     "tester",
		CountryCode:  "RU",
		CurrencyCode: "RUB",
	})
	require.True(t, errors.Is(err, domain.ErrUserAlreadyExists))
}

type mockRoundTripper func(*http.Request) (*http.Response, error)

func (m mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m(req)
}

func TestGoogleOAuth_Callback_Success(t *testing.T) {
	repo := &mockAuthRepository{
		getTempToken: func(_ context.Context, token string) (string, error) {
			require.Equal(t, "state123", token)
			return `{"code_verifier":"verifier123","nonce":"nonce123","created_at":1}`, nil
		},
		getUserByGoogleSub: func(_ context.Context, _ string) (*domain.User, error) {
			return nil, nil
		},
		getUserByEmail: func(_ context.Context, _ string) (*domain.User, error) {
			return nil, nil
		},
		createUserFromGoogle: func(_ context.Context, email, googleSub, username string) (*domain.User, error) {
			require.Equal(t, "new@example.com", email)
			require.Equal(t, "sub-1", googleSub)
			return &domain.User{
				ID:          "google-user-1",
				Email:       email,
				Username:    username,
				CountryCode: "US",
			}, nil
		},
	}
	svc := newTestService(repo)
	svc.ConfigureGoogleOAuth("client-id", "client-secret", "http://localhost:8080/api/v1/auth/google/callback", "http://localhost:3000")
	svc.httpClient = &http.Client{
		Transport: mockRoundTripper(func(req *http.Request) (*http.Response, error) {
			switch {
			case req.URL.String() == "https://oauth2.googleapis.com/token":
				body := `{"access_token":"ga","id_token":"id-token"}`
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			case strings.HasPrefix(req.URL.String(), "https://oauth2.googleapis.com/tokeninfo"):
				body := `{"sub":"sub-1","email":"new@example.com","email_verified":"true","aud":"client-id","nonce":"nonce123"}`
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			default:
				return nil, fmt.Errorf("unexpected URL: %s", req.URL.String())
			}
		}),
	}

	result, err := svc.LoginWithGoogleCallback(context.Background(), "auth-code", "state123", "127.0.0.1")
	require.NoError(t, err)
	require.Equal(t, "google-user-1", result.UserID)
	require.NotNil(t, result.Tokens)
	require.NotEmpty(t, result.Tokens.AccessToken)
}
