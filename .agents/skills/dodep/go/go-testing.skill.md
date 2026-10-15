# SKILL #20 — go-testing.skill.md

```markdown
# go-testing.skill.md
# GAMBLING PLATFORM — GO TESTING PATTERNS
# Version: 1.0.0 | Format: B (400-600 lines)
# Loaded by: Go Business Agent, QA Agent

# ============================================================
# SECTION 1: CONTEXT
# ============================================================

Go services: 80% coverage target.
Table-driven tests for logic with multiple scenarios.
Testcontainers for integration tests with real PostgreSQL/Redis.
Mocks generated with go.uber.org/mock or testify/mock.

# ============================================================
# SECTION 2: UNIT TEST PATTERNS
# ============================================================

```go
// ── Table-driven test (standard Go pattern) ──

func TestAuthService_Login(t *testing.T) {
    tests := []struct {
        name        string
        email       string
        password    string
        setupMock   func(*mockRepo.MockUserRepository)
        wantErr     error
        wantResult  bool
    }{
        {
            name:     "success",
            email:    "user@example.com",
            password: "correct_password",
            setupMock: func(m *mockRepo.MockUserRepository) {
                m.On("GetByEmail", mock.Anything, "user@example.com").
                    Return(fixtures.ActiveUserWithPassword("correct_password"), nil)
                m.On("ResetLoginAttempts", mock.Anything, mock.Anything).Return(nil)
                m.On("UpdateLastLogin", mock.Anything, mock.Anything).Return(nil)
            },
            wantErr:    nil,
            wantResult: true,
        },
        {
            name:     "user not found returns invalid credentials",
            email:    "nobody@example.com",
            password: "any",
            setupMock: func(m *mockRepo.MockUserRepository) {
                m.On("GetByEmail", mock.Anything, "nobody@example.com").
                    Return(nil, nil)
            },
            wantErr:    domain.ErrInvalidCredentials,
            wantResult: false,
        },
        {
            name:     "wrong password increments attempts",
            email:    "user@example.com",
            password: "wrong_password",
            setupMock: func(m *mockRepo.MockUserRepository) {
                m.On("GetByEmail", mock.Anything, "user@example.com").
                    Return(fixtures.ActiveUserWithPassword("correct_password"), nil)
                m.On("IncrementLoginAttempts", mock.Anything, mock.Anything).
                    Return(1, nil)
            },
            wantErr:    domain.ErrInvalidCredentials,
            wantResult: false,
        },
        {
            name:     "suspended account blocked",
            email:    "suspended@example.com",
            password: "any",
            setupMock: func(m *mockRepo.MockUserRepository) {
                m.On("GetByEmail", mock.Anything, "suspended@example.com").
                    Return(fixtures.SuspendedUser(), nil)
            },
            wantErr:    domain.ErrAccountSuspended,
            wantResult: false,
        },
        {
            name:     "self-excluded blocked",
            email:    "excluded@example.com",
            password: "any",
            setupMock: func(m *mockRepo.MockUserRepository) {
                m.On("GetByEmail", mock.Anything, "excluded@example.com").
                    Return(fixtures.SelfExcludedUser(), nil)
            },
            wantErr:    domain.ErrSelfExcluded,
            wantResult: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()

            userRepo := new(mockRepo.MockUserRepository)
            tt.setupMock(userRepo)

            svc := service.NewAuthService(userRepo, newTestTokenSvc(),
                newTestSessionSvc(), newTestProducer(), fixtures.DefaultConfig())

            result, err := svc.Login(context.Background(), service.LoginInput{
                Email:    tt.email,
                Password: tt.password,
            })

            if tt.wantErr != nil {
                assert.ErrorIs(t, err, tt.wantErr)
                assert.Nil(t, result)
            } else {
                require.NoError(t, err)
                assert.NotNil(t, result)
            }

            userRepo.AssertExpectations(t)
        })
    }
}
STATE MACHINE TESTS
Go

func TestUserStatus_CanTransitionTo(t *testing.T) {
    tests := []struct {
        from     domain.UserStatus
        to       domain.UserStatus
        allowed  bool
    }{
        {domain.UserStatusPending, domain.UserStatusActive, true},
        {domain.UserStatusActive, domain.UserStatusSuspended, true},
        {domain.UserStatusActive, domain.UserStatusBlocked, true},
        {domain.UserStatusActive, domain.UserStatusSelfExcluded, true},
        {domain.UserStatusSuspended, domain.UserStatusActive, true},
        {domain.UserStatusBlocked, domain.UserStatusActive, true},
        // Invalid
        {domain.UserStatusPending, domain.UserStatusBlocked, false},
        {domain.UserStatusClosed, domain.UserStatusActive, false},
        {domain.UserStatusSelfExcluded, domain.UserStatusSuspended, false},
    }

    for _, tt := range tests {
        name := fmt.Sprintf("%s_to_%s", tt.from, tt.to)
        t.Run(name, func(t *testing.T) {
            t.Parallel()
            assert.Equal(t, tt.allowed, tt.from.CanTransitionTo(tt.to))
        })
    }
}
============================================================
SECTION 3: INTEGRATION TESTS
============================================================
Go

//go:build integration

package integration

import (
    "context"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/modules/postgres"
    tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

type TestApp struct {
    DB    *gorm.DB
    Cache *redis.Client
    Svc   *service.AuthService
}

func setupTestApp(t *testing.T) *TestApp {
    ctx := context.Background()

    // Start PostgreSQL container
    pgContainer, err := postgres.Run(ctx,
        "postgres:16",
        postgres.WithDatabase("test"),
        postgres.WithUsername("test"),
        postgres.WithPassword("test"),
        testcontainers.WithWaitStrategy(wait.ForListeningPort("5432/tcp")),
    )
    require.NoError(t, err)
    t.Cleanup(func() { pgContainer.Terminate(ctx) })

    pgHost, _ := pgContainer.Host(ctx)
    pgPort, _ := pgContainer.MappedPort(ctx, "5432")

    db := connectDB(t, pgHost, pgPort.Int())
    runMigrations(t, db)

    // Start Redis container
    redisContainer, err := tcredis.Run(ctx, "redis:7")
    require.NoError(t, err)
    t.Cleanup(func() { redisContainer.Terminate(ctx) })

    redisEndpoint, _ := redisContainer.Endpoint(ctx, "")
    cache := connectRedis(t, redisEndpoint)

    // Build services
    svc := buildTestServices(db, cache)

    return &TestApp{DB: db, Cache: cache, Svc: svc}
}

func TestFullRegistrationFlow(t *testing.T) {
    app := setupTestApp(t)

    // Register
    result, err := app.Svc.Register(context.Background(), service.RegisterInput{
        Email:        "newuser@test.com",
        Password:     "SecureP@ss123!",
        CountryCode:  "DE",
        CurrencyCode: "EUR",
        AcceptTerms:  true,
        AgeConfirmed: true,
    })
    require.NoError(t, err)
    assert.NotZero(t, result.UserID)
    assert.NotEmpty(t, result.AccessToken)

    // Login with same credentials
    loginResult, err := app.Svc.Login(context.Background(), service.LoginInput{
        Email:    "newuser@test.com",
        Password: "SecureP@ss123!",
    })
    require.NoError(t, err)
    assert.NotEmpty(t, loginResult.AccessToken)

    // Verify duplicate registration fails
    _, err = app.Svc.Register(context.Background(), service.RegisterInput{
        Email:        "newuser@test.com",
        Password:     "Another123!",
        CountryCode:  "DE",
        CurrencyCode: "EUR",
        AcceptTerms:  true,
        AgeConfirmed: true,
    })
    assert.ErrorIs(t, err, domain.ErrEmailExists)
}

func TestLoginRateLimiting(t *testing.T) {
    app := setupTestApp(t)

    // Register user
    app.Svc.Register(context.Background(), service.RegisterInput{
        Email: "ratelimit@test.com", Password: "SecureP@ss123!",
        CountryCode: "DE", CurrencyCode: "EUR",
        AcceptTerms: true, AgeConfirmed: true,
    })

    // 10 failed attempts → account locked
    for i := 0; i < 10; i++ {
        _, err := app.Svc.Login(context.Background(), service.LoginInput{
            Email: "ratelimit@test.com", Password: "wrong",
        })
        assert.ErrorIs(t, err, domain.ErrInvalidCredentials)
    }

    // 11th attempt with correct password → still locked
    _, err := app.Svc.Login(context.Background(), service.LoginInput{
        Email: "ratelimit@test.com", Password: "SecureP@ss123!",
    })
    assert.ErrorIs(t, err, domain.ErrAccountLocked)
}
============================================================
SECTION 4: MOCK GENERATION
============================================================
Go

// Using testify/mock — define mock in tests/mocks/

type MockUserRepository struct {
    mock.Mock
}

func (m *MockUserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
    args := m.Called(ctx, email)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepository) Create(ctx context.Context, user *domain.User) error {
    args := m.Called(ctx, user)
    if args.Error(0) == nil {
        user.ID = 1 // simulate DB setting ID
    }
    return args.Error(0)
}

// ... implement all interface methods
============================================================
SECTION 5: FIXTURES
============================================================
Go

// tests/fixtures/users.go

package fixtures

func ActiveUser() *domain.User {
    return &domain.User{
        ID:       1,
        UUID:     uuid.New(),
        Email:    "active@example.com",
        Status:   domain.UserStatusActive,
        KYCLevel: 2,
        CountryCode: "DE",
    }
}

func ActiveUserWithPassword(password string) *domain.User {
    user := ActiveUser()
    user.PasswordHash = hashTestPassword(password)
    return user
}

func SuspendedUser() *domain.User {
    user := ActiveUser()
    user.Status = domain.UserStatusSuspended
    user.Email = "suspended@example.com"
    return user
}

func SelfExcludedUser() *domain.User {
    user := ActiveUser()
    user.Status = domain.UserStatusSelfExcluded
    user.Email = "excluded@example.com"
    return user
}

func DefaultConfig() config.AuthConfig {
    return config.AuthConfig{
        AccessTokenTTL:     15 * time.Minute,
        RefreshTokenTTL:    7 * 24 * time.Hour,
        MaxSessionsPerUser: 5,
        MaxLoginAttempts:   10,
        LockDuration:       30 * time.Minute,
    }
}
============================================================
SECTION 6: ANTI-PATTERNS
============================================================
text

❌ NEVER test private functions directly — test via public API
❌ NEVER share state between tests — each test independent
❌ NEVER use time.Sleep() — use channels or polling with deadline
❌ NEVER use production DB — use testcontainers
❌ NEVER skip error path tests — test every domain error
❌ NEVER write tests that depend on execution order
❌ NEVER mock everything — integration tests with real DB are valuable
❌ NEVER assert on string representations — use typed assertions
❌ NEVER skip t.Parallel() for independent tests
❌ NEVER ignore flaky tests — they indicate real bugs