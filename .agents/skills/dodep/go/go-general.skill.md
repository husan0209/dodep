


# SKILL #3 — go-general.skill.md

Базовый skill для всех Go-сервисов. Загружается каждым агентом, работающим с Go.

---

```markdown
# go-general.skill.md
# GAMBLING PLATFORM — GO GENERAL CONVENTIONS
# Version: 1.0.0
# Updated: 2025
# Loaded by: All Go agents
# Prerequisites: architecture-overview.skill.md

# ============================================================
# SECTION 1: ROLE AND CONTEXT
# ============================================================

## WHO YOU ARE

You are a Senior Go Backend Developer building the business logic
layer of a high-scale online gambling platform.

Your code handles:
- 50% of all platform code (most services)
- 18% of platform traffic (auth, payments, bonuses, KYC, etc.)
- All integrations with external providers (PSPs, KYC, etc.)
- All business workflows (registration, payments, bonuses)

Your code MUST be:
- Idiomatic Go (follow Effective Go, Go Proverbs)
- Simple and readable (no clever abstractions)
- Well-tested (table-driven tests)
- Properly structured (clean architecture)

## PERFORMANCE EXPECTATIONS

```text
Your services run on:
  CPU: 2-4 cores per instance
  RAM: 200-500 MB per instance
  Instances: 2-10 per service (auto-scaled)

Your code must achieve:
  p99 latency:        < 100ms for business operations
  p99 latency:        < 500ms for external API calls
  Throughput:          5,000+ requests/sec per instance
  Memory:             Stable over time (no goroutine leaks)
  Startup time:       < 3 seconds
  Graceful shutdown:  < 15 seconds (drain connections + finish jobs)
```

## KEY DIFFERENCE FROM RUST SERVICES

```text
Rust services = CRITICAL PATH (betting, wallet, odds, websocket)
  → Maximum performance, zero-copy, lock-free

Go services = BUSINESS LOGIC (auth, payments, bonus, KYC, CMS)
  → Development speed, maintainability, good-enough performance
  → More integrations with external APIs
  → More CRUD operations
  → More complex business workflows

If unsure which language:
  "Does it touch money in real-time?" → Rust
  "Does it handle 10K+ req/sec on single endpoint?" → Rust
  "Everything else" → Go
```

# ============================================================
# SECTION 2: PROJECT SETUP
# ============================================================

## GO MODULE

```go
// go.mod for a service
module github.com/platform/services/auth-service

go 1.23

require (
    // Internal shared library
    github.com/platform/libs/go-platform v0.1.0

    // Web framework
    github.com/gofiber/fiber/v2 v2.52.0
    github.com/gofiber/contrib/otelfiber v1.0.10

    // gRPC
    google.golang.org/grpc v1.65.0
    google.golang.org/protobuf v1.34.0

    // Database
    gorm.io/gorm v1.25.10
    gorm.io/driver/postgres v1.5.9

    // Cache
    github.com/redis/go-redis/v9 v9.5.0

    // Message broker
    github.com/twmb/franz-go v1.17.0
    github.com/twmb/franz-go/pkg/kadm v1.12.0

    // Observability
    go.opentelemetry.io/otel v1.28.0
    go.opentelemetry.io/otel/trace v1.28.0
    go.opentelemetry.io/otel/metric v1.28.0
    go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.28.0
    github.com/rs/zerolog v1.33.0
    github.com/prometheus/client_golang v1.19.0

    // Validation
    github.com/go-playground/validator/v10 v10.22.0

    // Auth
    github.com/golang-jwt/jwt/v5 v5.2.1
    golang.org/x/crypto v0.25.0

    // Config
    github.com/spf13/viper v1.19.0

    // Utils
    github.com/google/uuid v1.6.0
    github.com/shopspring/decimal v1.4.0
    github.com/samber/lo v1.39.0

    // Testing
    github.com/stretchr/testify v1.9.0
    github.com/testcontainers/testcontainers-go v0.32.0
    go.uber.org/mock v0.4.0

    // Error tracking
    github.com/getsentry/sentry-go v0.28.0
)
```

## PROJECT STRUCTURE

```text
services/auth-service/
├── go.mod
├── go.sum
├── Dockerfile
├── Makefile
├── README.md
│
├── cmd/
│   └── server/
│       └── main.go                 # Entry point
│
├── internal/                       # Private application code
│   ├── config/
│   │   └── config.go               # Configuration loading
│   │
│   ├── domain/                     # Core business types
│   │   ├── user.go                 # User entity
│   │   ├── session.go              # Session entity
│   │   ├── token.go                # Token value objects
│   │   └── errors.go               # Domain errors
│   │
│   ├── service/                    # Business logic (USE CASES)
│   │   ├── auth_service.go         # Authentication logic
│   │   ├── auth_service_test.go    # Unit tests
│   │   ├── token_service.go        # Token management
│   │   ├── token_service_test.go
│   │   ├── session_service.go      # Session management
│   │   └── session_service_test.go
│   │
│   ├── repository/                 # Data access layer
│   │   ├── interfaces.go           # Repository interfaces
│   │   ├── user_repo.go            # PostgreSQL implementation
│   │   ├── user_repo_test.go       # Integration tests
│   │   ├── session_repo.go         # DragonflyDB implementation
│   │   └── session_repo_test.go
│   │
│   ├── handler/                    # HTTP handlers (Fiber)
│   │   ├── auth_handler.go
│   │   ├── auth_handler_test.go
│   │   ├── session_handler.go
│   │   ├── middleware.go           # Auth middleware
│   │   └── response.go            # Response helpers
│   │
│   ├── grpc/                       # gRPC server
│   │   ├── server.go               # gRPC server setup
│   │   ├── auth_grpc.go            # gRPC service implementation
│   │   └── auth_grpc_test.go
│   │
│   ├── event/                      # Redpanda producer/consumer
│   │   ├── producer.go
│   │   └── consumer.go
│   │
│   └── client/                     # gRPC clients to other services
│       ├── user_client.go          # User service client
│       └── wallet_client.go        # Wallet service client
│
├── migrations/
│   ├── 000001_create_credentials.up.sql
│   ├── 000001_create_credentials.down.sql
│   ├── 000002_create_sessions.up.sql
│   └── 000002_create_sessions.down.sql
│
├── tests/
│   ├── integration/
│   │   ├── auth_flow_test.go       # Full auth flow tests
│   │   └── helpers_test.go         # Test helpers
│   └── fixtures/
│       └── fixtures.go             # Test data factories
│
└── config/
    ├── default.yaml
    ├── dev.yaml
    ├── staging.yaml
    └── production.yaml
```

## RULES FOR PROJECT STRUCTURE

```text
1. cmd/           — ONLY main.go, minimal bootstrap code
2. internal/      — ALL application code (Go enforces privacy)
3. domain/        — ZERO external dependencies (pure Go)
4. service/       — Business logic, depends on domain + repository interfaces
5. repository/    — Database access, implements interfaces from service
6. handler/       — HTTP/REST handlers, depends on service
7. grpc/          — gRPC handlers, depends on service
8. event/         — Redpanda producers/consumers
9. client/        — gRPC clients to other services

DEPENDENCY RULE:
  handler → service → repository (interfaces)
  handler NEVER imports repository
  service NEVER imports handler
  domain NEVER imports anything external

NAMING RULES:
  Files:        snake_case.go (user_repo.go, auth_service.go)
  Packages:     lowercase, single word (handler, service, domain)
  Interfaces:   -er suffix or descriptive (UserRepository, TokenGenerator)
  Structs:      PascalCase (AuthService, UserRepository)
  Methods:      PascalCase public, camelCase private
  Variables:    camelCase, short but meaningful
  Constants:    PascalCase (MaxLoginAttempts, DefaultTokenTTL)
  Errors:       ErrXxx (ErrUserNotFound, ErrInvalidCredentials)
  Test files:   xxx_test.go in same package
```

# ============================================================
# SECTION 3: APPLICATION BOOTSTRAP
# ============================================================

## MAIN.GO PATTERN

```go
// cmd/server/main.go
package main

import (
    "context"
    "fmt"
    "net"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/rs/zerolog"
    "github.com/rs/zerolog/log"
    "golang.org/x/sync/errgroup"
    "google.golang.org/grpc"

    "github.com/platform/services/auth-service/internal/config"
    "github.com/platform/services/auth-service/internal/handler"
    "github.com/platform/services/auth-service/internal/repository"
    "github.com/platform/services/auth-service/internal/service"
    authgrpc "github.com/platform/services/auth-service/internal/grpc"
    "github.com/platform/services/auth-service/internal/event"

    platformdb "github.com/platform/libs/go-platform/pkg/database"
    platformcache "github.com/platform/libs/go-platform/pkg/cache"
    platformevents "github.com/platform/libs/go-platform/pkg/events"
    platformtelemetry "github.com/platform/libs/go-platform/pkg/telemetry"
)

func main() {
    if err := run(); err != nil {
        log.Fatal().Err(err).Msg("Application failed")
    }
}

func run() error {
    // 1. Load configuration
    cfg, err := config.Load()
    if err != nil {
        return fmt.Errorf("load config: %w", err)
    }

    // 2. Initialize observability (BEFORE anything else)
    shutdown, err := platformtelemetry.Init(cfg.ServiceName, cfg.OTELEndpoint)
    if err != nil {
        return fmt.Errorf("init telemetry: %w", err)
    }
    defer shutdown(context.Background())

    // 3. Configure structured logging
    zerolog.TimeFieldFormat = time.RFC3339Nano
    if cfg.Environment == "dev" {
        log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
    }

    log.Info().
        Str("service", cfg.ServiceName).
        Str("version", cfg.Version).
        Str("environment", cfg.Environment).
        Msg("Starting service")

    // 4. Initialize dependencies
    db, err := platformdb.NewPool(cfg.Database)
    if err != nil {
        return fmt.Errorf("connect database: %w", err)
    }
    defer db.Close()

    cache, err := platformcache.NewClient(cfg.Cache)
    if err != nil {
        return fmt.Errorf("connect cache: %w", err)
    }
    defer cache.Close()

    producer, err := platformevents.NewProducer(cfg.Redpanda)
    if err != nil {
        return fmt.Errorf("create producer: %w", err)
    }
    defer producer.Close()

    // 5. Run database migrations
    if err := platformdb.RunMigrations(db, "migrations"); err != nil {
        return fmt.Errorf("run migrations: %w", err)
    }

    // 6. Build layers (dependency injection)
    repos := buildRepositories(db, cache)
    services := buildServices(cfg, repos, producer)
    handlers := handler.New(services)

    // 7. Create context with cancellation
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    g, ctx := errgroup.WithContext(ctx)

    // 8. Start HTTP server
    g.Go(func() error {
        return startHTTPServer(ctx, cfg, handlers)
    })

    // 9. Start gRPC server
    g.Go(func() error {
        return startGRPCServer(ctx, cfg, services)
    })

    // 10. Start event consumers
    g.Go(func() error {
        return startEventConsumer(ctx, cfg, services)
    })

    // 11. Wait for shutdown signal
    g.Go(func() error {
        return waitForShutdown(ctx, cancel)
    })

    log.Info().
        Int("http_port", cfg.HTTPPort).
        Int("grpc_port", cfg.GRPCPort).
        Msg("Service started")

    return g.Wait()
}

func waitForShutdown(ctx context.Context, cancel context.CancelFunc) error {
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

    select {
    case sig := <-quit:
        log.Info().Str("signal", sig.String()).Msg("Received shutdown signal")
    case <-ctx.Done():
        log.Info().Msg("Context cancelled")
    }

    cancel()
    return nil
}

func buildRepositories(db *platformdb.Pool, cache *platformcache.Client) *Repositories {
    return &Repositories{
        User:    repository.NewUserRepository(db),
        Session: repository.NewSessionRepository(cache),
    }
}

func buildServices(
    cfg *config.Config,
    repos *Repositories,
    producer *platformevents.Producer,
) *Services {
    tokenSvc := service.NewTokenService(cfg.Auth)
    sessionSvc := service.NewSessionService(repos.Session, cfg.Auth)
    authSvc := service.NewAuthService(
        repos.User,
        tokenSvc,
        sessionSvc,
        producer,
        cfg.Auth,
    )

    return &Services{
        Auth:    authSvc,
        Token:   tokenSvc,
        Session: sessionSvc,
    }
}

type Repositories struct {
    User    service.UserRepository
    Session service.SessionRepository
}

type Services struct {
    Auth    *service.AuthService
    Token   *service.TokenService
    Session *service.SessionService
}
```

## CONFIG PATTERN

```go
// internal/config/config.go
package config

import (
    "fmt"
    "time"

    "github.com/spf13/viper"

    platformdb "github.com/platform/libs/go-platform/pkg/database"
    platformcache "github.com/platform/libs/go-platform/pkg/cache"
    platformevents "github.com/platform/libs/go-platform/pkg/events"
)

type Config struct {
    ServiceName  string `mapstructure:"service_name"`
    Version      string `mapstructure:"version"`
    Environment  string `mapstructure:"environment"`
    HTTPPort     int    `mapstructure:"http_port"`
    GRPCPort     int    `mapstructure:"grpc_port"`
    OTELEndpoint string `mapstructure:"otel_endpoint"`

    Database platformdb.Config     `mapstructure:"database"`
    Cache    platformcache.Config  `mapstructure:"cache"`
    Redpanda platformevents.Config `mapstructure:"redpanda"`
    Auth     AuthConfig            `mapstructure:"auth"`
}

type AuthConfig struct {
    AccessTokenTTL     time.Duration `mapstructure:"access_token_ttl"`
    RefreshTokenTTL    time.Duration `mapstructure:"refresh_token_ttl"`
    MaxSessionsPerUser int           `mapstructure:"max_sessions_per_user"`
    MaxLoginAttempts   int           `mapstructure:"max_login_attempts"`
    LockDuration       time.Duration `mapstructure:"lock_duration"`
    Argon2Memory       uint32        `mapstructure:"argon2_memory"`
    Argon2Iterations   uint32        `mapstructure:"argon2_iterations"`
    Argon2Parallelism  uint8         `mapstructure:"argon2_parallelism"`
    Argon2SaltLength   uint32        `mapstructure:"argon2_salt_length"`
    Argon2KeyLength    uint32        `mapstructure:"argon2_key_length"`
    Ed25519PrivateKey  string        `mapstructure:"ed25519_private_key"`
    Ed25519PublicKey   string        `mapstructure:"ed25519_public_key"`
}

func Load() (*Config, error) {
    v := viper.New()

    // Defaults
    v.SetDefault("service_name", "auth-service")
    v.SetDefault("version", "0.1.0")
    v.SetDefault("environment", "dev")
    v.SetDefault("http_port", 8080)
    v.SetDefault("grpc_port", 9090)
    v.SetDefault("auth.access_token_ttl", "15m")
    v.SetDefault("auth.refresh_token_ttl", "168h") // 7 days
    v.SetDefault("auth.max_sessions_per_user", 5)
    v.SetDefault("auth.max_login_attempts", 10)
    v.SetDefault("auth.lock_duration", "30m")
    v.SetDefault("auth.argon2_memory", 65536)
    v.SetDefault("auth.argon2_iterations", 3)
    v.SetDefault("auth.argon2_parallelism", 4)
    v.SetDefault("auth.argon2_salt_length", 16)
    v.SetDefault("auth.argon2_key_length", 32)

    // Config file
    v.SetConfigName("default")
    v.AddConfigPath("config/")
    if err := v.ReadInConfig(); err != nil {
        return nil, fmt.Errorf("read default config: %w", err)
    }

    // Environment-specific overlay
    env := v.GetString("environment")
    v.SetConfigName(env)
    _ = v.MergeInConfig() // OK if missing

    // Environment variables override
    v.SetEnvPrefix("APP")
    v.AutomaticEnv()

    var cfg Config
    if err := v.Unmarshal(&cfg); err != nil {
        return nil, fmt.Errorf("unmarshal config: %w", err)
    }

    if err := cfg.validate(); err != nil {
        return nil, fmt.Errorf("validate config: %w", err)
    }

    return &cfg, nil
}

func (c *Config) validate() error {
    if c.HTTPPort <= 0 || c.HTTPPort > 65535 {
        return fmt.Errorf("invalid http_port: %d", c.HTTPPort)
    }
    if c.GRPCPort <= 0 || c.GRPCPort > 65535 {
        return fmt.Errorf("invalid grpc_port: %d", c.GRPCPort)
    }
    if c.HTTPPort == c.GRPCPort {
        return fmt.Errorf("http_port and grpc_port must be different")
    }
    if c.Auth.Ed25519PrivateKey == "" {
        return fmt.Errorf("ed25519_private_key is required")
    }
    return nil
}
```

# ============================================================
# SECTION 4: DOMAIN LAYER
# ============================================================

## RULES FOR DOMAIN

```text
1. Domain types are plain Go structs
2. Domain has ZERO external dependencies (no GORM, no Fiber)
3. Domain defines business errors as sentinel values
4. Use shopspring/decimal for ALL money values (NEVER float64)
5. Use time.Time for ALL timestamps
6. Use uuid.UUID for ALL external-facing IDs
7. Enums implemented as string constants with validation
8. Domain entities have validation methods
9. State machines are explicit with transition validation
```

## DOMAIN EXAMPLES

```go
// internal/domain/user.go
package domain

import (
    "fmt"
    "time"

    "github.com/google/uuid"
)

// ── User Entity ──

type User struct {
    ID            int64
    UUID          uuid.UUID
    Email         string
    Phone         *string
    PasswordHash  string
    Status        UserStatus
    KYCLevel      int
    CountryCode   string
    CurrencyCode  string
    CreatedAt     time.Time
    UpdatedAt     time.Time
    LastLoginAt   *time.Time
}

// ── User Status ──

type UserStatus string

const (
    UserStatusPending      UserStatus = "pending"
    UserStatusActive       UserStatus = "active"
    UserStatusSuspended    UserStatus = "suspended"
    UserStatusBlocked      UserStatus = "blocked"
    UserStatusSelfExcluded UserStatus = "self_excluded"
    UserStatusClosed       UserStatus = "closed"
)

func (s UserStatus) IsValid() bool {
    switch s {
    case UserStatusPending, UserStatusActive, UserStatusSuspended,
        UserStatusBlocked, UserStatusSelfExcluded, UserStatusClosed:
        return true
    }
    return false
}

func (s UserStatus) CanLogin() bool {
    return s == UserStatusActive
}

func (s UserStatus) CanTransitionTo(target UserStatus) bool {
    transitions := map[UserStatus][]UserStatus{
        UserStatusPending:      {UserStatusActive},
        UserStatusActive:       {UserStatusSuspended, UserStatusBlocked, UserStatusSelfExcluded, UserStatusClosed},
        UserStatusSuspended:    {UserStatusActive, UserStatusBlocked},
        UserStatusBlocked:      {UserStatusActive},
        UserStatusSelfExcluded: {UserStatusActive}, // only after exclusion period
    }

    allowed, ok := transitions[s]
    if !ok {
        return false
    }
    for _, a := range allowed {
        if a == target {
            return true
        }
    }
    return false
}

// ── Session ──

type Session struct {
    ID              string
    UserID          int64
    RefreshToken    string
    DeviceFingerprint string
    IPAddress       string
    UserAgent       string
    CreatedAt       time.Time
    LastActivityAt  time.Time
    ExpiresAt       time.Time
}

func (s *Session) IsExpired() bool {
    return time.Now().After(s.ExpiresAt)
}

// ── Token Claims ──

type TokenClaims struct {
    UserID      int64    `json:"sub"`
    Roles       []string `json:"roles"`
    Permissions []string `json:"permissions"`
    DeviceID    string   `json:"device_id"`
    TokenID     string   `json:"jti"`
    IssuedAt    int64    `json:"iat"`
    ExpiresAt   int64    `json:"exp"`
}
```

```go
// internal/domain/errors.go
package domain

import "errors"

// ── Sentinel Errors ──
// These are compared with errors.Is() in the handler layer
// to determine HTTP status codes.

// Authentication errors
var (
    ErrInvalidCredentials = errors.New("invalid email or password")
    ErrAccountLocked      = errors.New("account is temporarily locked")
    ErrAccountSuspended   = errors.New("account is suspended")
    ErrAccountNotActive   = errors.New("account is not active")
    ErrSelfExcluded       = errors.New("account is self-excluded")
    ErrTokenExpired       = errors.New("token has expired")
    ErrTokenInvalid       = errors.New("token is invalid")
    ErrTokenRevoked       = errors.New("token has been revoked")
    ErrSessionNotFound    = errors.New("session not found")
    ErrSessionLimitReached = errors.New("maximum sessions reached")
    Err2FARequired        = errors.New("two-factor authentication required")
    Err2FAInvalid         = errors.New("invalid 2FA code")
)

// User errors
var (
    ErrUserNotFound    = errors.New("user not found")
    ErrEmailExists     = errors.New("email already registered")
    ErrPhoneExists     = errors.New("phone already registered")
    ErrCountryBlocked  = errors.New("registration not available in this country")
    ErrUnderage        = errors.New("must be 18 or older")
)

// Wallet errors
var (
    ErrInsufficientBalance = errors.New("insufficient balance")
    ErrWalletLocked        = errors.New("wallet is locked")
    ErrCurrencyMismatch    = errors.New("currency mismatch")
)

// Generic errors
var (
    ErrNotFound     = errors.New("resource not found")
    ErrConflict     = errors.New("resource conflict")
    ErrForbidden    = errors.New("action forbidden")
    ErrRateLimited  = errors.New("rate limit exceeded")
)

// ── Detailed Errors (with context) ──

type ValidationError struct {
    Fields []FieldError
}

type FieldError struct {
    Field   string `json:"field"`
    Message string `json:"message"`
}

func (e *ValidationError) Error() string {
    if len(e.Fields) == 0 {
        return "validation failed"
    }
    return fmt.Sprintf("validation failed: %s — %s", e.Fields[0].Field, e.Fields[0].Message)
}

func NewValidationError(fields ...FieldError) *ValidationError {
    return &ValidationError{Fields: fields}
}

type DetailedError struct {
    Err     error
    Details map[string]interface{}
}

func (e *DetailedError) Error() string {
    return e.Err.Error()
}

func (e *DetailedError) Unwrap() error {
    return e.Err
}

func WithDetails(err error, details map[string]interface{}) *DetailedError {
    return &DetailedError{Err: err, Details: details}
}
```

# ============================================================
# SECTION 5: SERVICE LAYER
# ============================================================

## RULES FOR SERVICES

```text
1. Service contains ALL business logic
2. Service depends on INTERFACES (not concrete repos)
3. Service validates BUSINESS RULES
4. Service orchestrates repository calls + external services
5. Service publishes domain events via Redpanda producer
6. Service returns domain errors (handler maps to HTTP)
7. Service methods accept context.Context as first parameter
8. Service NEVER knows about HTTP (no fiber.Ctx, no StatusCode)
9. Service uses structured logging (zerolog)
10. Service constructor validates dependencies (fail fast)
```

## REPOSITORY INTERFACES (defined alongside service)

```go
// internal/repository/interfaces.go
package repository

import (
    "context"

    "github.com/platform/services/auth-service/internal/domain"
)

// UserRepository defines data access for users.
// Implementation is in user_repo.go (PostgreSQL).
// This interface is used by services for dependency injection.
type UserRepository interface {
    // Create inserts a new user. Returns ErrEmailExists if duplicate.
    Create(ctx context.Context, user *domain.User) error

    // GetByID returns a user by internal ID.
    GetByID(ctx context.Context, id int64) (*domain.User, error)

    // GetByEmail returns a user by email. Returns nil, nil if not found.
    GetByEmail(ctx context.Context, email string) (*domain.User, error)

    // GetByPhone returns a user by phone. Returns nil, nil if not found.
    GetByPhone(ctx context.Context, phone string) (*domain.User, error)

    // Update updates user fields. Uses optimistic locking.
    Update(ctx context.Context, user *domain.User) error

    // UpdateStatus transitions user status. Validates transition.
    UpdateStatus(ctx context.Context, id int64, from, to domain.UserStatus) error

    // UpdateLastLogin sets last_login_at to now.
    UpdateLastLogin(ctx context.Context, id int64) error

    // IncrementLoginAttempts increments failed login counter.
    IncrementLoginAttempts(ctx context.Context, id int64) (attempts int, err error)

    // ResetLoginAttempts resets failed login counter to 0.
    ResetLoginAttempts(ctx context.Context, id int64) error
}

// SessionRepository defines data access for sessions.
// Implementation is in session_repo.go (DragonflyDB).
type SessionRepository interface {
    // Create stores a new session. Enforces max sessions per user.
    Create(ctx context.Context, session *domain.Session) error

    // GetByRefreshToken returns session by refresh token hash.
    GetByRefreshToken(ctx context.Context, tokenHash string) (*domain.Session, error)

    // GetUserSessions returns all active sessions for a user.
    GetUserSessions(ctx context.Context, userID int64) ([]*domain.Session, error)

    // Delete removes a session (logout).
    Delete(ctx context.Context, sessionID string) error

    // DeleteAllForUser removes all sessions (force logout).
    DeleteAllForUser(ctx context.Context, userID int64) error

    // CountUserSessions returns active session count.
    CountUserSessions(ctx context.Context, userID int64) (int, error)
}
```

## SERVICE EXAMPLE — AuthService

```go
// internal/service/auth_service.go
package service

import (
    "context"
    "crypto/subtle"
    "errors"
    "fmt"
    "time"

    "github.com/google/uuid"
    "github.com/rs/zerolog/log"

    "github.com/platform/services/auth-service/internal/config"
    "github.com/platform/services/auth-service/internal/domain"
    "github.com/platform/services/auth-service/internal/repository"
    platformevents "github.com/platform/libs/go-platform/pkg/events"
)

type AuthService struct {
    userRepo   repository.UserRepository
    tokenSvc   *TokenService
    sessionSvc *SessionService
    producer   *platformevents.Producer
    cfg        config.AuthConfig
}

func NewAuthService(
    userRepo repository.UserRepository,
    tokenSvc *TokenService,
    sessionSvc *SessionService,
    producer *platformevents.Producer,
    cfg config.AuthConfig,
) *AuthService {
    if userRepo == nil {
        panic("userRepo is required")
    }
    if tokenSvc == nil {
        panic("tokenSvc is required")
    }
    if sessionSvc == nil {
        panic("sessionSvc is required")
    }
    if producer == nil {
        panic("producer is required")
    }

    return &AuthService{
        userRepo:   userRepo,
        tokenSvc:   tokenSvc,
        sessionSvc: sessionSvc,
        producer:   producer,
        cfg:        cfg,
    }
}

// ── Registration ──

type RegisterInput struct {
    Email        string
    Password     string
    CountryCode  string
    CurrencyCode string
    AcceptTerms  bool
    AgeConfirmed bool
    IPAddress    string
    UserAgent    string
}

type RegisterOutput struct {
    UserID       int64
    AccessToken  string
    RefreshToken string
}

func (s *AuthService) Register(ctx context.Context, input RegisterInput) (*RegisterOutput, error) {
    logger := log.Ctx(ctx).With().
        Str("method", "AuthService.Register").
        Str("email", maskEmail(input.Email)).
        Str("country", input.CountryCode).
        Logger()

    // 1. Business validation
    if !input.AcceptTerms {
        return nil, domain.NewValidationError(
            domain.FieldError{Field: "accept_terms", Message: "Terms must be accepted"},
        )
    }
    if !input.AgeConfirmed {
        return nil, domain.NewValidationError(
            domain.FieldError{Field: "age_confirmed", Message: "Age confirmation required"},
        )
    }
    if isBlockedCountry(input.CountryCode) {
        return nil, domain.ErrCountryBlocked
    }

    // 2. Check email uniqueness
    existing, err := s.userRepo.GetByEmail(ctx, input.Email)
    if err != nil {
        return nil, fmt.Errorf("check email: %w", err)
    }
    if existing != nil {
        return nil, domain.ErrEmailExists
    }

    // 3. Hash password
    hash, err := hashPassword(input.Password, s.cfg)
    if err != nil {
        return nil, fmt.Errorf("hash password: %w", err)
    }

    // 4. Create user
    user := &domain.User{
        UUID:         uuid.New(),
        Email:        input.Email,
        PasswordHash: hash,
        Status:       domain.UserStatusPending, // until email verified
        KYCLevel:     0,
        CountryCode:  input.CountryCode,
        CurrencyCode: input.CurrencyCode,
        CreatedAt:    time.Now(),
        UpdatedAt:    time.Now(),
    }

    if err := s.userRepo.Create(ctx, user); err != nil {
        if errors.Is(err, domain.ErrEmailExists) {
            return nil, domain.ErrEmailExists
        }
        return nil, fmt.Errorf("create user: %w", err)
    }

    // 5. Generate tokens
    accessToken, err := s.tokenSvc.GenerateAccessToken(user)
    if err != nil {
        return nil, fmt.Errorf("generate access token: %w", err)
    }

    refreshToken, err := s.tokenSvc.GenerateRefreshToken()
    if err != nil {
        return nil, fmt.Errorf("generate refresh token: %w", err)
    }

    // 6. Create session
    session := &domain.Session{
        ID:                uuid.New().String(),
        UserID:            user.ID,
        RefreshToken:      hashToken(refreshToken),
        DeviceFingerprint: "", // from request
        IPAddress:         input.IPAddress,
        UserAgent:         input.UserAgent,
        CreatedAt:         time.Now(),
        LastActivityAt:    time.Now(),
        ExpiresAt:         time.Now().Add(s.cfg.RefreshTokenTTL),
    }

    if err := s.sessionSvc.Create(ctx, session); err != nil {
        return nil, fmt.Errorf("create session: %w", err)
    }

    // 7. Publish event
    if err := s.producer.Publish(ctx, "users.registered", fmt.Sprintf("%d", user.ID), &UserRegisteredEvent{
        UserID:      user.ID,
        Email:       user.Email,
        CountryCode: user.CountryCode,
        RegisteredAt: user.CreatedAt,
    }); err != nil {
        // Non-critical — log and continue
        logger.Error().Err(err).Msg("Failed to publish registration event")
    }

    logger.Info().Int64("user_id", user.ID).Msg("User registered successfully")

    return &RegisterOutput{
        UserID:       user.ID,
        AccessToken:  accessToken,
        RefreshToken: refreshToken,
    }, nil
}

// ── Login ──

type LoginInput struct {
    Email             string
    Password          string
    DeviceFingerprint string
    IPAddress         string
    UserAgent         string
}

type LoginOutput struct {
    AccessToken  string
    RefreshToken string
    Requires2FA  bool
    TempToken    string // only if 2FA required
}

func (s *AuthService) Login(ctx context.Context, input LoginInput) (*LoginOutput, error) {
    logger := log.Ctx(ctx).With().
        Str("method", "AuthService.Login").
        Str("email", maskEmail(input.Email)).
        Logger()

    // 1. Get user by email
    user, err := s.userRepo.GetByEmail(ctx, input.Email)
    if err != nil {
        return nil, fmt.Errorf("get user: %w", err)
    }
    if user == nil {
        // Constant time comparison to prevent timing attacks
        dummyVerify()
        return nil, domain.ErrInvalidCredentials
    }

    // 2. Check account status
    if !user.Status.CanLogin() {
        switch user.Status {
        case domain.UserStatusBlocked:
            return nil, domain.ErrAccountLocked
        case domain.UserStatusSuspended:
            return nil, domain.ErrAccountSuspended
        case domain.UserStatusSelfExcluded:
            return nil, domain.ErrSelfExcluded
        default:
            return nil, domain.ErrAccountNotActive
        }
    }

    // 3. Verify password
    if !verifyPassword(input.Password, user.PasswordHash) {
        // Increment failed attempts
        attempts, _ := s.userRepo.IncrementLoginAttempts(ctx, user.ID)
        if attempts >= s.cfg.MaxLoginAttempts {
            _ = s.userRepo.UpdateStatus(ctx, user.ID, domain.UserStatusActive, domain.UserStatusBlocked)
            logger.Warn().Int("attempts", attempts).Msg("Account locked due to failed attempts")

            // Publish event for fraud monitoring
            _ = s.producer.Publish(ctx, "auth.account_locked", fmt.Sprintf("%d", user.ID), &AccountLockedEvent{
                UserID:    user.ID,
                Reason:    "max_login_attempts",
                IPAddress: input.IPAddress,
            })

            return nil, domain.ErrAccountLocked
        }

        // Publish failed login event
        _ = s.producer.Publish(ctx, "auth.failed_login", fmt.Sprintf("%d", user.ID), &FailedLoginEvent{
            UserID:    user.ID,
            IPAddress: input.IPAddress,
            UserAgent: input.UserAgent,
            Attempt:   attempts,
        })

        return nil, domain.ErrInvalidCredentials
    }

    // 4. Reset failed login attempts
    _ = s.userRepo.ResetLoginAttempts(ctx, user.ID)

    // 5. Check if 2FA is enabled
    // TODO: implement 2FA check
    // if user.Has2FA {
    //     tempToken := s.tokenSvc.GenerateTempToken(user)
    //     return &LoginOutput{Requires2FA: true, TempToken: tempToken}, nil
    // }

    // 6. Generate tokens
    accessToken, err := s.tokenSvc.GenerateAccessToken(user)
    if err != nil {
        return nil, fmt.Errorf("generate access token: %w", err)
    }

    refreshToken, err := s.tokenSvc.GenerateRefreshToken()
    if err != nil {
        return nil, fmt.Errorf("generate refresh token: %w", err)
    }

    // 7. Create session (check max sessions)
    sessionCount, err := s.sessionSvc.repo.CountUserSessions(ctx, user.ID)
    if err != nil {
        return nil, fmt.Errorf("count sessions: %w", err)
    }
    if sessionCount >= s.cfg.MaxSessionsPerUser {
        // Remove oldest session
        sessions, _ := s.sessionSvc.repo.GetUserSessions(ctx, user.ID)
        if len(sessions) > 0 {
            _ = s.sessionSvc.repo.Delete(ctx, sessions[0].ID)
        }
    }

    session := &domain.Session{
        ID:                uuid.New().String(),
        UserID:            user.ID,
        RefreshToken:      hashToken(refreshToken),
        DeviceFingerprint: input.DeviceFingerprint,
        IPAddress:         input.IPAddress,
        UserAgent:         input.UserAgent,
        CreatedAt:         time.Now(),
        LastActivityAt:    time.Now(),
        ExpiresAt:         time.Now().Add(s.cfg.RefreshTokenTTL),
    }

    if err := s.sessionSvc.Create(ctx, session); err != nil {
        return nil, fmt.Errorf("create session: %w", err)
    }

    // 8. Update last login
    _ = s.userRepo.UpdateLastLogin(ctx, user.ID)

    // 9. Publish event
    _ = s.producer.Publish(ctx, "auth.login", fmt.Sprintf("%d", user.ID), &UserLoggedInEvent{
        UserID:    user.ID,
        IPAddress: input.IPAddress,
        Device:    input.DeviceFingerprint,
    })

    logger.Info().Int64("user_id", user.ID).Msg("User logged in successfully")

    return &LoginOutput{
        AccessToken:  accessToken,
        RefreshToken: refreshToken,
        Requires2FA:  false,
    }, nil
}

// ── Token Refresh ──

type RefreshInput struct {
    RefreshToken string
    IPAddress    string
    UserAgent    string
}

func (s *AuthService) RefreshTokens(ctx context.Context, input RefreshInput) (*LoginOutput, error) {
    // 1. Find session by refresh token hash
    session, err := s.sessionSvc.repo.GetByRefreshToken(ctx, hashToken(input.RefreshToken))
    if err != nil {
        return nil, fmt.Errorf("get session: %w", err)
    }
    if session == nil {
        return nil, domain.ErrTokenInvalid
    }

    // 2. Check expiration
    if session.IsExpired() {
        _ = s.sessionSvc.repo.Delete(ctx, session.ID)
        return nil, domain.ErrTokenExpired
    }

    // 3. Get user
    user, err := s.userRepo.GetByID(ctx, session.UserID)
    if err != nil {
        return nil, fmt.Errorf("get user: %w", err)
    }
    if user == nil || !user.Status.CanLogin() {
        _ = s.sessionSvc.repo.Delete(ctx, session.ID)
        return nil, domain.ErrAccountNotActive
    }

    // 4. Generate new token pair (ROTATION)
    accessToken, err := s.tokenSvc.GenerateAccessToken(user)
    if err != nil {
        return nil, fmt.Errorf("generate access token: %w", err)
    }

    newRefreshToken, err := s.tokenSvc.GenerateRefreshToken()
    if err != nil {
        return nil, fmt.Errorf("generate refresh token: %w", err)
    }

    // 5. Update session with new refresh token
    session.RefreshToken = hashToken(newRefreshToken)
    session.LastActivityAt = time.Now()
    session.ExpiresAt = time.Now().Add(s.cfg.RefreshTokenTTL)

    if err := s.sessionSvc.Update(ctx, session); err != nil {
        return nil, fmt.Errorf("update session: %w", err)
    }

    return &LoginOutput{
        AccessToken:  accessToken,
        RefreshToken: newRefreshToken,
    }, nil
}

// ── Logout ──

func (s *AuthService) Logout(ctx context.Context, sessionID string) error {
    return s.sessionSvc.repo.Delete(ctx, sessionID)
}

// ── Helpers ──

func maskEmail(email string) string {
    at := 0
    for i, c := range email {
        if c == '@' {
            at = i
            break
        }
    }
    if at <= 1 {
        return "***"
    }
    return string(email[0]) + "***" + email[at:]
}

func isBlockedCountry(code string) bool {
    blocked := map[string]bool{
        "US": true, "GB": true, "FR": true, "AU": true,
        // ... add per license requirements
    }
    return blocked[code]
}

// dummyVerify prevents timing attacks on user enumeration
func dummyVerify() {
    _ = verifyPassword("dummy", "$argon2id$v=19$m=65536,t=3,p=4$dummysalt$dummyhash")
}
```

# ============================================================
# SECTION 6: HANDLER LAYER
# ============================================================

## RULES FOR HANDLERS

```text
1. Handler is THIN — extract input, call service, return output
2. Handler validates REQUEST FORMAT (struct tags, validator)
3. Handler maps domain errors to HTTP status codes
4. Handler NEVER contains business logic
5. Handler NEVER calls repository directly
6. Handler uses Fiber context for request/response
7. Handler function takes (c *fiber.Ctx) error
8. All handlers are methods on a Handler struct
9. Handler logs at debug level only (service logs at info)
10. Handler adds request_id to response
```

## HANDLER EXAMPLE

```go
// internal/handler/auth_handler.go
package handler

import (
    "errors"

    "github.com/gofiber/fiber/v2"
    "github.com/go-playground/validator/v10"

    "github.com/platform/services/auth-service/internal/domain"
    "github.com/platform/services/auth-service/internal/service"
)

type Handler struct {
    auth    *service.AuthService
    token   *service.TokenService
    session *service.SessionService
    validate *validator.Validate
}

func New(services *Services) *Handler {
    return &Handler{
        auth:     services.Auth,
        token:    services.Token,
        session:  services.Session,
        validate: validator.New(),
    }
}

// ── Request/Response DTOs ──

type RegisterRequest struct {
    Email        string `json:"email" validate:"required,email,max=255"`
    Password     string `json:"password" validate:"required,min=8,max=128"`
    CountryCode  string `json:"country_code" validate:"required,len=2,alpha"`
    CurrencyCode string `json:"currency_code" validate:"required,len=3,alpha"`
    AcceptTerms  bool   `json:"accept_terms" validate:"required"`
    AgeConfirmed bool   `json:"age_confirmed" validate:"required"`
}

type LoginRequest struct {
    Email             string `json:"email" validate:"required,email"`
    Password          string `json:"password" validate:"required"`
    DeviceFingerprint string `json:"device_fingerprint" validate:"max=64"`
}

type RefreshRequest struct {
    RefreshToken string `json:"refresh_token" validate:"required"`
}

type AuthResponse struct {
    UserID       int64  `json:"user_id,omitempty"`
    AccessToken  string `json:"access_token"`
    RefreshToken string `json:"refresh_token"`
    Requires2FA  bool   `json:"requires_2fa,omitempty"`
    TempToken    string `json:"temp_token,omitempty"`
}

// ── Handlers ──

// Register creates a new user account.
//
//	@Summary      Register new user
//	@Description  Create a new user account with email and password
//	@Tags         auth
//	@Accept       json
//	@Produce      json
//	@Param        request body RegisterRequest true "Registration data"
//	@Success      201 {object} SuccessResponse{data=AuthResponse}
//	@Failure      400 {object} ErrorResponse
//	@Failure      409 {object} ErrorResponse
//	@Failure      429 {object} ErrorResponse
//	@Router       /api/v1/auth/register [post]
func (h *Handler) Register(c *fiber.Ctx) error {
    var req RegisterRequest
    if err := c.BodyParser(&req); err != nil {
        return respondError(c, fiber.StatusBadRequest, "INVALID_BODY", "Invalid request body")
    }

    if err := h.validate.Struct(req); err != nil {
        return respondValidationError(c, err.(validator.ValidationErrors))
    }

    result, err := h.auth.Register(c.Context(), service.RegisterInput{
        Email:        req.Email,
        Password:     req.Password,
        CountryCode:  req.CountryCode,
        CurrencyCode: req.CurrencyCode,
        AcceptTerms:  req.AcceptTerms,
        AgeConfirmed: req.AgeConfirmed,
        IPAddress:    c.IP(),
        UserAgent:    c.Get("User-Agent"),
    })
    if err != nil {
        return h.mapError(c, err)
    }

    return respondSuccess(c, fiber.StatusCreated, AuthResponse{
        UserID:       result.UserID,
        AccessToken:  result.AccessToken,
        RefreshToken: result.RefreshToken,
    })
}

// Login authenticates a user.
func (h *Handler) Login(c *fiber.Ctx) error {
    var req LoginRequest
    if err := c.BodyParser(&req); err != nil {
        return respondError(c, fiber.StatusBadRequest, "INVALID_BODY", "Invalid request body")
    }

    if err := h.validate.Struct(req); err != nil {
        return respondValidationError(c, err.(validator.ValidationErrors))
    }

    result, err := h.auth.Login(c.Context(), service.LoginInput{
        Email:             req.Email,
        Password:          req.Password,
        DeviceFingerprint: req.DeviceFingerprint,
        IPAddress:         c.IP(),
        UserAgent:         c.Get("User-Agent"),
    })
    if err != nil {
        return h.mapError(c, err)
    }

    return respondSuccess(c, fiber.StatusOK, AuthResponse{
        AccessToken:  result.AccessToken,
        RefreshToken: result.RefreshToken,
        Requires2FA:  result.Requires2FA,
        TempToken:    result.TempToken,
    })
}

// RefreshTokens issues new access and refresh tokens.
func (h *Handler) RefreshTokens(c *fiber.Ctx) error {
    var req RefreshRequest
    if err := c.BodyParser(&req); err != nil {
        return respondError(c, fiber.StatusBadRequest, "INVALID_BODY", "Invalid request body")
    }

    if err := h.validate.Struct(req); err != nil {
        return respondValidationError(c, err.(validator.ValidationErrors))
    }

    result, err := h.auth.RefreshTokens(c.Context(), service.RefreshInput{
        RefreshToken: req.RefreshToken,
        IPAddress:    c.IP(),
        UserAgent:    c.Get("User-Agent"),
    })
    if err != nil {
        return h.mapError(c, err)
    }

    return respondSuccess(c, fiber.StatusOK, AuthResponse{
        AccessToken:  result.AccessToken,
        RefreshToken: result.RefreshToken,
    })
}

// Logout invalidates the current session.
func (h *Handler) Logout(c *fiber.Ctx) error {
    user, ok := c.Locals("user").(*domain.TokenClaims)
    if !ok {
        return respondError(c, fiber.StatusUnauthorized, "AUTH_UNAUTHORIZED", "Not authenticated")
    }

    if err := h.auth.Logout(c.Context(), user.TokenID); err != nil {
        return h.mapError(c, err)
    }

    return c.SendStatus(fiber.StatusNoContent)
}
```

## ERROR MAPPING

```go
// internal/handler/response.go
package handler

import (
    "errors"
    "time"

    "github.com/gofiber/fiber/v2"
    "github.com/go-playground/validator/v10"
    "github.com/rs/zerolog/log"

    "github.com/platform/services/auth-service/internal/domain"
)

// ── Standard Response Envelopes ──

type SuccessResponse struct {
    Data interface{} `json:"data"`
    Meta Meta        `json:"meta"`
}

type ErrorResponse struct {
    Error ErrorBody `json:"error"`
    Meta  Meta      `json:"meta"`
}

type ErrorBody struct {
    Code    string      `json:"code"`
    Message string      `json:"message"`
    Details interface{} `json:"details,omitempty"`
}

type Meta struct {
    RequestID string `json:"request_id"`
    Timestamp string `json:"timestamp"`
}

type PaginatedResponse struct {
    Data       interface{} `json:"data"`
    Pagination Pagination  `json:"pagination"`
    Meta       Meta        `json:"meta"`
}

type Pagination struct {
    Cursor     string `json:"cursor,omitempty"`
    HasMore    bool   `json:"has_more"`
    TotalCount int64  `json:"total_count,omitempty"`
}

// ── Response Helpers ──

func respondSuccess(c *fiber.Ctx, status int, data interface{}) error {
    return c.Status(status).JSON(SuccessResponse{
        Data: data,
        Meta: buildMeta(c),
    })
}

func respondError(c *fiber.Ctx, status int, code, message string) error {
    return c.Status(status).JSON(ErrorResponse{
        Error: ErrorBody{Code: code, Message: message},
        Meta:  buildMeta(c),
    })
}

func respondErrorWithDetails(c *fiber.Ctx, status int, code, message string, details interface{}) error {
    return c.Status(status).JSON(ErrorResponse{
        Error: ErrorBody{Code: code, Message: message, Details: details},
        Meta:  buildMeta(c),
    })
}

func respondValidationError(c *fiber.Ctx, errs validator.ValidationErrors) error {
    fields := make([]domain.FieldError, 0, len(errs))
    for _, e := range errs {
        fields = append(fields, domain.FieldError{
            Field:   e.Field(),
            Message: validationMessage(e),
        })
    }
    return respondErrorWithDetails(c, fiber.StatusBadRequest, "VALIDATION_FAILED", "Validation failed", fields)
}

func buildMeta(c *fiber.Ctx) Meta {
    return Meta{
        RequestID: c.GetRespHeader("X-Request-ID", ""),
        Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
    }
}

func validationMessage(e validator.FieldError) string {
    switch e.Tag() {
    case "required":
        return "This field is required"
    case "email":
        return "Must be a valid email address"
    case "min":
        return "Must be at least " + e.Param() + " characters"
    case "max":
        return "Must be at most " + e.Param() + " characters"
    case "len":
        return "Must be exactly " + e.Param() + " characters"
    default:
        return "Invalid value"
    }
}

// ── Error Mapping ──

// mapError converts domain errors to HTTP responses.
// This is the ONLY place where domain errors are mapped to HTTP status codes.
func (h *Handler) mapError(c *fiber.Ctx, err error) error {
    // Validation errors
    var validationErr *domain.ValidationError
    if errors.As(err, &validationErr) {
        return respondErrorWithDetails(c, fiber.StatusBadRequest,
            "VALIDATION_FAILED", "Validation failed", validationErr.Fields)
    }

    // Detailed errors (unwrap to get sentinel + details)
    var detailedErr *domain.DetailedError
    if errors.As(err, &detailedErr) {
        err = detailedErr.Err // use inner error for matching below
    }

    // Sentinel error mapping
    switch {
    // 401 Unauthorized
    case errors.Is(err, domain.ErrInvalidCredentials):
        return respondError(c, fiber.StatusUnauthorized, "AUTH_INVALID_CREDENTIALS", "Invalid email or password")
    case errors.Is(err, domain.ErrTokenExpired):
        return respondError(c, fiber.StatusUnauthorized, "AUTH_TOKEN_EXPIRED", "Token has expired")
    case errors.Is(err, domain.ErrTokenInvalid):
        return respondError(c, fiber.StatusUnauthorized, "AUTH_TOKEN_INVALID", "Invalid token")
    case errors.Is(err, domain.ErrTokenRevoked):
        return respondError(c, fiber.StatusUnauthorized, "AUTH_TOKEN_REVOKED", "Token has been revoked")

    // 403 Forbidden
    case errors.Is(err, domain.ErrAccountLocked):
        return respondError(c, fiber.StatusForbidden, "AUTH_ACCOUNT_LOCKED", "Account is temporarily locked")
    case errors.Is(err, domain.ErrAccountSuspended):
        return respondError(c, fiber.StatusForbidden, "AUTH_ACCOUNT_SUSPENDED", "Account is suspended")
    case errors.Is(err, domain.ErrSelfExcluded):
        return respondError(c, fiber.StatusForbidden, "AUTH_SELF_EXCLUDED", "Account is self-excluded from gambling")
    case errors.Is(err, domain.ErrAccountNotActive):
        return respondError(c, fiber.StatusForbidden, "AUTH_ACCOUNT_NOT_ACTIVE", "Account is not active")
    case errors.Is(err, domain.ErrForbidden):
        return respondError(c, fiber.StatusForbidden, "FORBIDDEN", "Action not allowed")
    case errors.Is(err, domain.ErrCountryBlocked):
        return respondError(c, fiber.StatusForbidden, "USER_COUNTRY_BLOCKED", "Service not available in your country")

    // 404 Not Found
    case errors.Is(err, domain.ErrNotFound),
        errors.Is(err, domain.ErrUserNotFound),
        errors.Is(err, domain.ErrSessionNotFound):
        return respondError(c, fiber.StatusNotFound, "NOT_FOUND", "Resource not found")

    // 409 Conflict
    case errors.Is(err, domain.ErrEmailExists):
        return respondError(c, fiber.StatusConflict, "USER_EMAIL_EXISTS", "Email is already registered")
    case errors.Is(err, domain.ErrPhoneExists):
        return respondError(c, fiber.StatusConflict, "USER_PHONE_EXISTS", "Phone is already registered")
    case errors.Is(err, domain.ErrConflict):
        return respondError(c, fiber.StatusConflict, "CONFLICT", "Resource conflict")

    // 422 Unprocessable
    case errors.Is(err, domain.Err2FARequired):
        return respondError(c, fiber.StatusUnprocessableEntity, "AUTH_2FA_REQUIRED", "Two-factor authentication required")
    case errors.Is(err, domain.Err2FAInvalid):
        return respondError(c, fiber.StatusUnprocessableEntity, "AUTH_2FA_INVALID", "Invalid 2FA code")
    case errors.Is(err, domain.ErrSessionLimitReached):
        return respondError(c, fiber.StatusUnprocessableEntity, "AUTH_SESSION_LIMIT", "Maximum active sessions reached")

    // 429 Rate Limited
    case errors.Is(err, domain.ErrRateLimited):
        c.Set("Retry-After", "60")
        return respondError(c, fiber.StatusTooManyRequests, "RATE_LIMITED", "Too many requests")

    // 500 Internal (NEVER expose details)
    default:
        log.Error().Err(err).
            Str("request_id", c.GetRespHeader("X-Request-ID")).
            Msg("Internal server error")
        return respondError(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", "An internal error occurred")
    }
}
```

# ============================================================
# SECTION 7: REPOSITORY IMPLEMENTATION
# ============================================================

## POSTGRESQL REPOSITORY

```go
// internal/repository/user_repo.go
package repository

import (
    "context"
    "errors"
    "fmt"
    "time"

    "github.com/jackc/pgx/v5/pgconn"
    "gorm.io/gorm"

    "github.com/platform/services/auth-service/internal/domain"
)

type userRepository struct {
    db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
    return &userRepository{db: db}
}

// ── GORM Model (separate from domain entity) ──

type userModel struct {
    ID            int64          `gorm:"primaryKey;autoIncrement"`
    UUID          string         `gorm:"type:uuid;uniqueIndex;not null"`
    Email         string         `gorm:"type:varchar(255);uniqueIndex;not null"`
    Phone         *string        `gorm:"type:varchar(20);uniqueIndex"`
    PasswordHash  string         `gorm:"type:varchar(255);not null"`
    Status        string         `gorm:"type:user_status;not null;default:'pending'"`
    KYCLevel      int            `gorm:"type:smallint;not null;default:0"`
    CountryCode   string         `gorm:"type:char(2);not null"`
    CurrencyCode  string         `gorm:"type:char(3);not null"`
    LoginAttempts int            `gorm:"type:int;not null;default:0"`
    CreatedAt     time.Time      `gorm:"autoCreateTime"`
    UpdatedAt     time.Time      `gorm:"autoUpdateTime"`
    LastLoginAt   *time.Time
}

func (userModel) TableName() string { return "users" }

// ── Conversion ──

func (m *userModel) toDomain() *domain.User {
    return &domain.User{
        ID:           m.ID,
        UUID:         uuid.MustParse(m.UUID),
        Email:        m.Email,
        Phone:        m.Phone,
        PasswordHash: m.PasswordHash,
        Status:       domain.UserStatus(m.Status),
        KYCLevel:     m.KYCLevel,
        CountryCode:  m.CountryCode,
        CurrencyCode: m.CurrencyCode,
        CreatedAt:    m.CreatedAt,
        UpdatedAt:    m.UpdatedAt,
        LastLoginAt:  m.LastLoginAt,
    }
}

func userFromDomain(u *domain.User) *userModel {
    return &userModel{
        ID:           u.ID,
        UUID:         u.UUID.String(),
        Email:        u.Email,
        Phone:        u.Phone,
        PasswordHash: u.PasswordHash,
        Status:       string(u.Status),
        KYCLevel:     u.KYCLevel,
        CountryCode:  u.CountryCode,
        CurrencyCode: u.CurrencyCode,
    }
}

// ── Implementation ──

func (r *userRepository) Create(ctx context.Context, user *domain.User) error {
    model := userFromDomain(user)

    result := r.db.WithContext(ctx).Create(model)
    if result.Error != nil {
        // Check for unique constraint violation
        var pgErr *pgconn.PgError
        if errors.As(result.Error, &pgErr) && pgErr.Code == "23505" {
            if pgErr.ConstraintName == "idx_users_email" {
                return domain.ErrEmailExists
            }
            if pgErr.ConstraintName == "idx_users_phone" {
                return domain.ErrPhoneExists
            }
        }
        return fmt.Errorf("insert user: %w", result.Error)
    }

    // Set generated fields back on domain entity
    user.ID = model.ID
    user.CreatedAt = model.CreatedAt
    user.UpdatedAt = model.UpdatedAt

    return nil
}

func (r *userRepository) GetByID(ctx context.Context, id int64) (*domain.User, error) {
    var model userModel
    result := r.db.WithContext(ctx).Where("id = ?", id).First(&model)
    if result.Error != nil {
        if errors.Is(result.Error, gorm.ErrRecordNotFound) {
            return nil, nil // not found is not an error
        }
        return nil, fmt.Errorf("get user by id: %w", result.Error)
    }
    return model.toDomain(), nil
}

func (r *userRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
    var model userModel
    result := r.db.WithContext(ctx).Where("email = ?", email).First(&model)
    if result.Error != nil {
        if errors.Is(result.Error, gorm.ErrRecordNotFound) {
            return nil, nil
        }
        return nil, fmt.Errorf("get user by email: %w", result.Error)
    }
    return model.toDomain(), nil
}

func (r *userRepository) GetByPhone(ctx context.Context, phone string) (*domain.User, error) {
    var model userModel
    result := r.db.WithContext(ctx).Where("phone = ?", phone).First(&model)
    if result.Error != nil {
        if errors.Is(result.Error, gorm.ErrRecordNotFound) {
            return nil, nil
        }
        return nil, fmt.Errorf("get user by phone: %w", result.Error)
    }
    return model.toDomain(), nil
}

func (r *userRepository) Update(ctx context.Context, user *domain.User) error {
    result := r.db.WithContext(ctx).
        Model(&userModel{}).
        Where("id = ?", user.ID).
        Updates(map[string]interface{}{
            "email":         user.Email,
            "phone":         user.Phone,
            "status":        string(user.Status),
            "kyc_level":     user.KYCLevel,
            "country_code":  user.CountryCode,
            "currency_code": user.CurrencyCode,
            "updated_at":    time.Now(),
        })
    if result.Error != nil {
        return fmt.Errorf("update user: %w", result.Error)
    }
    if result.RowsAffected == 0 {
        return domain.ErrUserNotFound
    }
    return nil
}

func (r *userRepository) UpdateStatus(ctx context.Context, id int64, from, to domain.UserStatus) error {
    result := r.db.WithContext(ctx).
        Model(&userModel{}).
        Where("id = ? AND status = ?", id, string(from)).
        Update("status", string(to))
    if result.Error != nil {
        return fmt.Errorf("update status: %w", result.Error)
    }
    if result.RowsAffected == 0 {
        return domain.ErrConflict // status was already changed
    }
    return nil
}

func (r *userRepository) UpdateLastLogin(ctx context.Context, id int64) error {
    now := time.Now()
    return r.db.WithContext(ctx).
        Model(&userModel{}).
        Where("id = ?", id).
        Update("last_login_at", now).Error
}

func (r *userRepository) IncrementLoginAttempts(ctx context.Context, id int64) (int, error) {
    var attempts int
    err := r.db.WithContext(ctx).Raw(
        "UPDATE users SET login_attempts = login_attempts + 1 WHERE id = ? RETURNING login_attempts",
        id,
    ).Scan(&attempts).Error
    return attempts, err
}

func (r *userRepository) ResetLoginAttempts(ctx context.Context, id int64) error {
    return r.db.WithContext(ctx).
        Model(&userModel{}).
        Where("id = ?", id).
        Update("login_attempts", 0).Error
}
```

## DRAGONFLYDB (CACHE) REPOSITORY

```go
// internal/repository/session_repo.go
package repository

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/redis/go-redis/v9"

    "github.com/platform/services/auth-service/internal/domain"
)

type sessionRepository struct {
    client *redis.Client
}

func NewSessionRepository(client *redis.Client) SessionRepository {
    return &sessionRepository{client: client}
}

const (
    sessionPrefix     = "session:"
    sessionUserPrefix = "session:user:"
    refreshPrefix     = "refresh:"
)

func (r *sessionRepository) Create(ctx context.Context, session *domain.Session) error {
    data, err := json.Marshal(session)
    if err != nil {
        return fmt.Errorf("marshal session: %w", err)
    }

    ttl := time.Until(session.ExpiresAt)
    pipe := r.client.Pipeline()

    // Store session by ID
    pipe.Set(ctx, sessionPrefix+session.ID, data, ttl)

    // Index: refresh token hash → session ID
    pipe.Set(ctx, refreshPrefix+session.RefreshToken, session.ID, ttl)

    // Index: user sessions set
    pipe.SAdd(ctx, sessionUserPrefix+fmt.Sprintf("%d", session.UserID), session.ID)
    pipe.Expire(ctx, sessionUserPrefix+fmt.Sprintf("%d", session.UserID), ttl)

    _, err = pipe.Exec(ctx)
    if err != nil {
        return fmt.Errorf("create session: %w", err)
    }

    return nil
}

func (r *sessionRepository) GetByRefreshToken(ctx context.Context, tokenHash string) (*domain.Session, error) {
    // Get session ID from refresh token index
    sessionID, err := r.client.Get(ctx, refreshPrefix+tokenHash).Result()
    if err != nil {
        if err == redis.Nil {
            return nil, nil
        }
        return nil, fmt.Errorf("get refresh index: %w", err)
    }

    // Get session data
    data, err := r.client.Get(ctx, sessionPrefix+sessionID).Result()
    if err != nil {
        if err == redis.Nil {
            return nil, nil
        }
        return nil, fmt.Errorf("get session: %w", err)
    }

    var session domain.Session
    if err := json.Unmarshal([]byte(data), &session); err != nil {
        return nil, fmt.Errorf("unmarshal session: %w", err)
    }

    return &session, nil
}

func (r *sessionRepository) GetUserSessions(ctx context.Context, userID int64) ([]*domain.Session, error) {
    key := sessionUserPrefix + fmt.Sprintf("%d", userID)
    sessionIDs, err := r.client.SMembers(ctx, key).Result()
    if err != nil {
        return nil, fmt.Errorf("get user sessions: %w", err)
    }

    sessions := make([]*domain.Session, 0, len(sessionIDs))
    for _, id := range sessionIDs {
        data, err := r.client.Get(ctx, sessionPrefix+id).Result()
        if err != nil {
            if err == redis.Nil {
                // Session expired, clean up set
                r.client.SRem(ctx, key, id)
                continue
            }
            return nil, fmt.Errorf("get session %s: %w", id, err)
        }

        var session domain.Session
        if err := json.Unmarshal([]byte(data), &session); err != nil {
            continue
        }
        sessions = append(sessions, &session)
    }

    return sessions, nil
}

func (r *sessionRepository) Delete(ctx context.Context, sessionID string) error {
    // Get session to find user ID and refresh token
    data, err := r.client.Get(ctx, sessionPrefix+sessionID).Result()
    if err != nil {
        if err == redis.Nil {
            return nil // already deleted
        }
        return fmt.Errorf("get session for delete: %w", err)
    }

    var session domain.Session
    if err := json.Unmarshal([]byte(data), &session); err != nil {
        return fmt.Errorf("unmarshal session: %w", err)
    }

    pipe := r.client.Pipeline()
    pipe.Del(ctx, sessionPrefix+sessionID)
    pipe.Del(ctx, refreshPrefix+session.RefreshToken)
    pipe.SRem(ctx, sessionUserPrefix+fmt.Sprintf("%d", session.UserID), sessionID)

    _, err = pipe.Exec(ctx)
    return err
}

func (r *sessionRepository) DeleteAllForUser(ctx context.Context, userID int64) error {
    sessions, err := r.GetUserSessions(ctx, userID)
    if err != nil {
        return err
    }

    for _, session := range sessions {
        if err := r.Delete(ctx, session.ID); err != nil {
            return err
        }
    }

    return nil
}

func (r *sessionRepository) CountUserSessions(ctx context.Context, userID int64) (int, error) {
    count, err := r.client.SCard(ctx, sessionUserPrefix+fmt.Sprintf("%d", userID)).Result()
    if err != nil {
        return 0, fmt.Errorf("count sessions: %w", err)
    }
    return int(count), nil
}
```

# ============================================================
# SECTION 8: MIDDLEWARE
# ============================================================

```go
// internal/handler/middleware.go
package handler

import (
    "strings"
    "time"

    "github.com/gofiber/fiber/v2"
    "github.com/google/uuid"
    "github.com/rs/zerolog/log"

    "github.com/platform/services/auth-service/internal/domain"
    "github.com/platform/services/auth-service/internal/service"
)

// RequestID adds a unique request ID to every request.
func RequestID() fiber.Handler {
    return func(c *fiber.Ctx) error {
        id := c.Get("X-Request-ID")
        if id == "" {
            id = uuid.New().String()
        }
        c.Set("X-Request-ID", id)
        c.Locals("request_id", id)
        return c.Next()
    }
}

// Logger logs every request with structured fields.
func Logger() fiber.Handler {
    return func(c *fiber.Ctx) error {
        start := time.Now()

        err := c.Next()

        duration := time.Since(start)
        logger := log.With().
            Str("request_id", c.Locals("request_id").(string)).
            Str("method", c.Method()).
            Str("path", c.Path()).
            Int("status", c.Response().StatusCode()).
            Dur("duration", duration).
            Str("ip", c.IP()).
            Logger()

        if c.Response().StatusCode() >= 500 {
            logger.Error().Msg("Request failed")
        } else if c.Response().StatusCode() >= 400 {
            logger.Warn().Msg("Request error")
        } else {
            logger.Info().Msg("Request completed")
        }

        return err
    }
}

// RequireAuth validates JWT token and sets user in context.
func RequireAuth(tokenSvc *service.TokenService) fiber.Handler {
    return func(c *fiber.Ctx) error {
        // 1. Extract token from Authorization header
        auth := c.Get("Authorization")
        if auth == "" {
            return respondError(c, fiber.StatusUnauthorized,
                "AUTH_UNAUTHORIZED", "Authorization header required")
        }

        if !strings.HasPrefix(auth, "Bearer ") {
            return respondError(c, fiber.StatusUnauthorized,
                "AUTH_UNAUTHORIZED", "Bearer token required")
        }

        token := strings.TrimPrefix(auth, "Bearer ")

        // 2. Validate token
        claims, err := tokenSvc.ValidateAccessToken(token)
        if err != nil {
            return respondError(c, fiber.StatusUnauthorized,
                "AUTH_TOKEN_INVALID", "Invalid or expired token")
        }

        // 3. Set claims in context for handlers
        c.Locals("user", claims)
        c.Locals("user_id", claims.UserID)

        return c.Next()
    }
}

// RequireRole checks if user has required role.
func RequireRole(roles ...string) fiber.Handler {
    return func(c *fiber.Ctx) error {
        claims, ok := c.Locals("user").(*domain.TokenClaims)
        if !ok {
            return respondError(c, fiber.StatusUnauthorized,
                "AUTH_UNAUTHORIZED", "Not authenticated")
        }

        for _, required := range roles {
            for _, userRole := range claims.Roles {
                if userRole == required {
                    return c.Next()
                }
            }
        }

        return respondError(c, fiber.StatusForbidden,
            "AUTH_FORBIDDEN", "Insufficient permissions")
    }
}

// Recover catches panics and returns 500.
func Recover() fiber.Handler {
    return func(c *fiber.Ctx) error {
        defer func() {
            if r := recover(); r != nil {
                log.Error().
                    Interface("panic", r).
                    Str("request_id", c.Locals("request_id").(string)).
                    Msg("Panic recovered")

                _ = respondError(c, fiber.StatusInternalServerError,
                    "INTERNAL_ERROR", "An internal error occurred")
            }
        }()
        return c.Next()
    }
}
```

# ============================================================
# SECTION 9: ROUTER SETUP
# ============================================================

```go
// internal/handler/router.go
package handler

import (
    "github.com/gofiber/fiber/v2"
    "github.com/gofiber/fiber/v2/middleware/limiter"
    "github.com/gofiber/contrib/otelfiber"
    "time"
)

func SetupRoutes(app *fiber.App, h *Handler, tokenSvc *service.TokenService) {
    // Global middleware
    app.Use(Recover())
    app.Use(RequestID())
    app.Use(Logger())
    app.Use(otelfiber.Middleware()) // OpenTelemetry tracing

    // Health checks (no auth)
    app.Get("/healthz", h.Liveness)
    app.Get("/readyz", h.Readiness)
    app.Get("/metrics", h.Metrics)

    // API v1
    v1 := app.Group("/api/v1")

    // Auth routes (no auth required)
    auth := v1.Group("/auth")
    auth.Post("/register", rateLimiter(5, time.Hour), h.Register)
    auth.Post("/login", rateLimiter(10, time.Minute), h.Login)
    auth.Post("/refresh", rateLimiter(30, time.Minute), h.RefreshTokens)
    auth.Post("/forgot-password", rateLimiter(3, time.Hour), h.ForgotPassword)
    auth.Post("/reset-password", rateLimiter(5, time.Hour), h.ResetPassword)

    // Protected auth routes
    authProtected := auth.Group("", RequireAuth(tokenSvc))
    authProtected.Post("/logout", h.Logout)
    authProtected.Post("/2fa/enable", h.Enable2FA)
    authProtected.Post("/2fa/verify", h.Verify2FA)
    authProtected.Post("/2fa/disable", h.Disable2FA)

    // Session management (authenticated)
    sessions := v1.Group("/sessions", RequireAuth(tokenSvc))
    sessions.Get("/", h.ListSessions)
    sessions.Delete("/:session_id", h.RevokeSession)
}

func rateLimiter(max int, window time.Duration) fiber.Handler {
    return limiter.New(limiter.Config{
        Max:        max,
        Expiration: window,
        KeyGenerator: func(c *fiber.Ctx) string {
            return c.IP() // rate limit by IP
        },
        LimitReached: func(c *fiber.Ctx) error {
            c.Set("Retry-After", fmt.Sprintf("%d", int(window.Seconds())))
            return respondError(c, fiber.StatusTooManyRequests,
                "RATE_LIMITED", "Too many requests")
        },
    })
}
```

# ============================================================
# SECTION 10: TESTING
# ============================================================

## RULES FOR TESTING

```text
1. Table-driven tests for all logic with multiple cases
2. Use testify/assert for assertions
3. Use testify/mock or go.uber.org/mock for mocking
4. Use testcontainers for integration tests
5. Test files in same package (access to unexported fields)
6. Integration tests use build tag: //go:build integration
7. Test naming: Test{Function}_{Scenario}_{Expected}
8. Each test is independent (no shared state between tests)
9. Use t.Parallel() where possible
10. NEVER sleep in tests — use channels, waitgroups, or polling
```

## UNIT TEST EXAMPLE

```go
// internal/service/auth_service_test.go
package service

import (
    "context"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
    "github.com/stretchr/testify/require"

    "github.com/platform/services/auth-service/internal/domain"
    "github.com/platform/services/auth-service/tests/fixtures"
    mockRepo "github.com/platform/services/auth-service/tests/mocks"
)

func TestAuthService_Register_Success(t *testing.T) {
    t.Parallel()

    // Arrange
    userRepo := new(mockRepo.MockUserRepository)
    tokenSvc := newTestTokenService()
    sessionSvc := newTestSessionService()
    producer := newTestProducer()
    cfg := fixtures.DefaultAuthConfig()

    svc := NewAuthService(userRepo, tokenSvc, sessionSvc, producer, cfg)

    userRepo.On("GetByEmail", mock.Anything, "test@example.com").Return(nil, nil)
    userRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.User")).
        Run(func(args mock.Arguments) {
            user := args.Get(1).(*domain.User)
            user.ID = 1 // simulate DB setting ID
        }).
        Return(nil)

    input := RegisterInput{
        Email:        "test@example.com",
        Password:     "SecureP@ss123",
        CountryCode:  "DE",
        CurrencyCode: "EUR",
        AcceptTerms:  true,
        AgeConfirmed: true,
        IPAddress:    "1.2.3.4",
        UserAgent:    "TestAgent",
    }

    // Act
    result, err := svc.Register(context.Background(), input)

    // Assert
    require.NoError(t, err)
    assert.Equal(t, int64(1), result.UserID)
    assert.NotEmpty(t, result.AccessToken)
    assert.NotEmpty(t, result.RefreshToken)

    userRepo.AssertExpectations(t)
}

func TestAuthService_Register_EmailExists(t *testing.T) {
    t.Parallel()

    userRepo := new(mockRepo.MockUserRepository)
    svc := NewAuthService(userRepo, newTestTokenService(), newTestSessionService(),
        newTestProducer(), fixtures.DefaultAuthConfig())

    existingUser := &domain.User{ID: 1, Email: "test@example.com"}
    userRepo.On("GetByEmail", mock.Anything, "test@example.com").Return(existingUser, nil)

    input := RegisterInput{
        Email:        "test@example.com",
        Password:     "SecureP@ss123",
        CountryCode:  "DE",
        CurrencyCode: "EUR",
        AcceptTerms:  true,
        AgeConfirmed: true,
    }

    result, err := svc.Register(context.Background(), input)

    assert.Nil(t, result)
    assert.ErrorIs(t, err, domain.ErrEmailExists)
}

func TestAuthService_Register_BlockedCountry(t *testing.T) {
    t.Parallel()

    svc := NewAuthService(new(mockRepo.MockUserRepository), newTestTokenService(),
        newTestSessionService(), newTestProducer(), fixtures.DefaultAuthConfig())

    input := RegisterInput{
        Email:        "test@example.com",
        Password:     "SecureP@ss123",
        CountryCode:  "US", // blocked
        CurrencyCode: "USD",
        AcceptTerms:  true,
        AgeConfirmed: true,
    }

    result, err := svc.Register(context.Background(), input)

    assert.Nil(t, result)
    assert.ErrorIs(t, err, domain.ErrCountryBlocked)
}

// Table-driven test for login scenarios
func TestAuthService_Login(t *testing.T) {
    t.Parallel()

    tests := []struct {
        name          string
        setupMock     func(*mockRepo.MockUserRepository)
        input         LoginInput
        expectError   error
        expectResult  bool
    }{
        {
            name: "successful login",
            setupMock: func(m *mockRepo.MockUserRepository) {
                user := fixtures.ActiveUser()
                m.On("GetByEmail", mock.Anything, user.Email).Return(user, nil)
                m.On("ResetLoginAttempts", mock.Anything, user.ID).Return(nil)
                m.On("UpdateLastLogin", mock.Anything, user.ID).Return(nil)
            },
            input: LoginInput{
                Email:    "active@example.com",
                Password: "CorrectPassword123!",
            },
            expectError:  nil,
            expectResult: true,
        },
        {
            name: "user not found returns invalid credentials",
            setupMock: func(m *mockRepo.MockUserRepository) {
                m.On("GetByEmail", mock.Anything, "noone@example.com").Return(nil, nil)
            },
            input: LoginInput{
                Email:    "noone@example.com",
                Password: "AnyPassword123!",
            },
            expectError:  domain.ErrInvalidCredentials,
            expectResult: false,
        },
        {
            name: "wrong password increments attempts",
            setupMock: func(m *mockRepo.MockUserRepository) {
                user := fixtures.ActiveUser()
                m.On("GetByEmail", mock.Anything, user.Email).Return(user, nil)
                m.On("IncrementLoginAttempts", mock.Anything, user.ID).Return(1, nil)
            },
            input: LoginInput{
                Email:    "active@example.com",
                Password: "WrongPassword123!",
            },
            expectError:  domain.ErrInvalidCredentials,
            expectResult: false,
        },
        {
            name: "suspended account returns error",
            setupMock: func(m *mockRepo.MockUserRepository) {
                user := fixtures.SuspendedUser()
                m.On("GetByEmail", mock.Anything, user.Email).Return(user, nil)
            },
            input: LoginInput{
                Email:    "suspended@example.com",
                Password: "AnyPassword123!",
            },
            expectError:  domain.ErrAccountSuspended,
            expectResult: false,
        },
        {
            name: "self-excluded account returns error",
            setupMock: func(m *mockRepo.MockUserRepository) {
                user := fixtures.SelfExcludedUser()
                m.On("GetByEmail", mock.Anything, user.Email).Return(user, nil)
            },
            input: LoginInput{
                Email:    "excluded@example.com",
                Password: "AnyPassword123!",
            },
            expectError:  domain.ErrSelfExcluded,
            expectResult: false,
        },
    }

    for _, tt := range tests {
        tt := tt // capture range variable
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()

            userRepo := new(mockRepo.MockUserRepository)
            tt.setupMock(userRepo)

            svc := NewAuthService(userRepo, newTestTokenService(),
                newTestSessionService(), newTestProducer(), fixtures.DefaultAuthConfig())

            result, err := svc.Login(context.Background(), tt.input)

            if tt.expectError != nil {
                assert.ErrorIs(t, err, tt.expectError)
                assert.Nil(t, result)
            } else {
                require.NoError(t, err)
                assert.NotNil(t, result)
            }

            userRepo.AssertExpectations(t)
        })
    }
}

func TestUserStatus_CanTransitionTo(t *testing.T) {
    t.Parallel()

    tests := []struct {
        from     domain.UserStatus
        to       domain.UserStatus
        expected bool
    }{
        {domain.UserStatusPending, domain.UserStatusActive, true},
        {domain.UserStatusActive, domain.UserStatusSuspended, true},
        {domain.UserStatusActive, domain.UserStatusBlocked, true},
        {domain.UserStatusActive, domain.UserStatusSelfExcluded, true},
        {domain.UserStatusSuspended, domain.UserStatusActive, true},
        {domain.UserStatusBlocked, domain.UserStatusActive, true},

        // Invalid transitions
        {domain.UserStatusPending, domain.UserStatusBlocked, false},
        {domain.UserStatusClosed, domain.UserStatusActive, false},
        {domain.UserStatusSelfExcluded, domain.UserStatusSuspended, false},
    }

    for _, tt := range tests {
        tt := tt
        t.Run(fmt.Sprintf("%s_to_%s", tt.from, tt.to), func(t *testing.T) {
            t.Parallel()
            assert.Equal(t, tt.expected, tt.from.CanTransitionTo(tt.to))
        })
    }
}
```

## INTEGRATION TEST EXAMPLE

```go
//go:build integration

// tests/integration/auth_flow_test.go
package integration

import (
    "context"
    "net/http"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/modules/postgres"
    "github.com/testcontainers/testcontainers-go/modules/redis"
)

func TestFullAuthFlow(t *testing.T) {
    ctx := context.Background()

    // Start containers
    pgContainer, db := startPostgres(t, ctx)
    defer pgContainer.Terminate(ctx)

    redisContainer, cache := startRedis(t, ctx)
    defer redisContainer.Terminate(ctx)

    // Setup application
    app := setupTestApp(t, db, cache)
    defer app.Shutdown()

    // 1. Register
    registerResp := httpPost(t, app, "/api/v1/auth/register", map[string]interface{}{
        "email":         "integration@test.com",
        "password":      "SecureP@ss123!",
        "country_code":  "DE",
        "currency_code": "EUR",
        "accept_terms":  true,
        "age_confirmed": true,
    })
    assert.Equal(t, http.StatusCreated, registerResp.StatusCode)
    assert.NotEmpty(t, registerResp.Body.Data["access_token"])
    assert.NotEmpty(t, registerResp.Body.Data["refresh_token"])

    // 2. Login
    loginResp := httpPost(t, app, "/api/v1/auth/login", map[string]interface{}{
        "email":    "integration@test.com",
        "password": "SecureP@ss123!",
    })
    assert.Equal(t, http.StatusOK, loginResp.StatusCode)
    accessToken := loginResp.Body.Data["access_token"].(string)
    refreshToken := loginResp.Body.Data["refresh_token"].(string)

    // 3. Refresh token
    refreshResp := httpPost(t, app, "/api/v1/auth/refresh", map[string]interface{}{
        "refresh_token": refreshToken,
    })
    assert.Equal(t, http.StatusOK, refreshResp.StatusCode)
    newAccessToken := refreshResp.Body.Data["access_token"].(string)
    assert.NotEqual(t, accessToken, newAccessToken) // new token issued

    // 4. Old refresh token should be invalid (rotation)
    refreshResp2 := httpPost(t, app, "/api/v1/auth/refresh", map[string]interface{}{
        "refresh_token": refreshToken, // old token
    })
    assert.Equal(t, http.StatusUnauthorized, refreshResp2.StatusCode)

    // 5. Logout
    logoutResp := httpPostWithAuth(t, app, "/api/v1/auth/logout", nil, newAccessToken)
    assert.Equal(t, http.StatusNoContent, logoutResp.StatusCode)

    // 6. Token should be invalid after logout
    sessionsResp := httpGetWithAuth(t, app, "/api/v1/sessions", newAccessToken)
    // Token itself is still valid (JWT), but session was deleted
    // In a more complete implementation, we'd check session validity
}

func TestLoginRateLimit(t *testing.T) {
    ctx := context.Background()
    pgContainer, db := startPostgres(t, ctx)
    defer pgContainer.Terminate(ctx)
    redisContainer, cache := startRedis(t, ctx)
    defer redisContainer.Terminate(ctx)
    app := setupTestApp(t, db, cache)
    defer app.Shutdown()

    // Make 11 login attempts (limit is 10/minute)
    for i := 0; i < 10; i++ {
        resp := httpPost(t, app, "/api/v1/auth/login", map[string]interface{}{
            "email":    "doesnotexist@test.com",
            "password": "anything",
        })
        assert.Equal(t, http.StatusUnauthorized, resp.StatusCode) // invalid creds, but not rate limited
    }

    // 11th request should be rate limited
    resp := httpPost(t, app, "/api/v1/auth/login", map[string]interface{}{
        "email":    "doesnotexist@test.com",
        "password": "anything",
    })
    assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
    assert.NotEmpty(t, resp.Header.Get("Retry-After"))
}
```

# ============================================================
# SECTION 11: DOCKERFILE
# ============================================================

```dockerfile
# services/auth-service/Dockerfile

# ── Stage 1: Build ──
FROM golang:1.23-bookworm AS builder

WORKDIR /app

# Copy go.mod files first (cache dependencies)
COPY services/auth-service/go.mod services/auth-service/go.sum ./services/auth-service/
COPY libs/go-platform/go.mod libs/go-platform/go.sum ./libs/go-platform/

WORKDIR /app/services/auth-service
RUN go mod download

# Copy source code
WORKDIR /app
COPY libs/go-platform/ ./libs/go-platform/
COPY services/auth-service/ ./services/auth-service/

# Build
WORKDIR /app/services/auth-service
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.version=$(cat VERSION 2>/dev/null || echo dev)" \
    -o /app/bin/auth-service \
    ./cmd/server

# ── Stage 2: Runtime ──
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /app/bin/auth-service /app/auth-service
COPY services/auth-service/config/ /app/config/
COPY services/auth-service/migrations/ /app/migrations/

WORKDIR /app

EXPOSE 8080 9090

USER nonroot:nonroot

ENTRYPOINT ["/app/auth-service"]
```

# ============================================================
# SECTION 12: ANTI-PATTERNS SUMMARY
# ============================================================

```text
❌ NEVER use float64 for money
   ✅ Use shopspring/decimal.Decimal

❌ NEVER use panic() for error handling (only in init/main for unrecoverable)
   ✅ Return error and handle at caller

❌ NEVER ignore errors (_, _ = someFunc())
   ✅ Handle or explicitly comment: _ = cache.Del(ctx, key) // best-effort

❌ NEVER use naked goroutines (go func() { ... })
   ✅ Use errgroup, or at minimum recover + logging

❌ NEVER use global variables for state
   ✅ Pass dependencies via constructors (dependency injection)

❌ NEVER use init() functions for complex setup
   ✅ Do setup in main() or constructors

❌ NEVER use interface{} / any without reason
   ✅ Use concrete types or generic constraints

❌ NEVER log sensitive data (passwords, tokens, card numbers)
   ✅ Mask: email → j***@example.com, phone → ***1234

❌ NEVER use fmt.Errorf without %w for wrapped errors
   ✅ return fmt.Errorf("get user: %w", err)

❌ NEVER use string concatenation for SQL queries
   ✅ Use parameterized queries: WHERE id = ?

❌ NEVER use context.TODO() in production code
   ✅ Propagate context from handler through all layers

❌ NEVER create goroutines without cancellation
   ✅ Always accept context.Context for cancellation

❌ NEVER use sync.Mutex where you need concurrent map access
   ✅ Use sync.Map or concurrent-safe structures

❌ NEVER return concrete types from constructors
   ✅ Return interfaces: func NewUserRepo(db) UserRepository

❌ NEVER put business logic in handlers
   ✅ Handler → Service → Repository (clean architecture)

❌ NEVER share GORM models across packages
   ✅ Convert to/from domain types at repository boundary

❌ NEVER use time.Sleep in production code
   ✅ Use tickers, timers, or context with deadline

❌ NEVER hardcode timeouts
   ✅ Use configuration values from config struct

❌ NEVER skip checking error from Close(), Commit(), Rollback()
   ✅ defer func() { if err := tx.Rollback(); err != nil { log... } }()
```

# ============================================================
# SECTION 13: MAKEFILE
# ============================================================

```makefile
# services/auth-service/Makefile

.PHONY: build run test lint migrate docker

SERVICE_NAME := auth-service
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

# Build
build:
	CGO_ENABLED=0 go build -ldflags="-w -s -X main.version=$(VERSION)" \
		-o bin/$(SERVICE_NAME) ./cmd/server

# Run locally
run:
	ENVIRONMENT=dev go run ./cmd/server

# Tests
test:
	go test -v -race -count=1 ./internal/...

test-integration:
	go test -v -race -count=1 -tags=integration ./tests/integration/...

test-coverage:
	go test -v -race -coverprofile=coverage.out ./internal/...
	go tool cover -html=coverage.out -o coverage.html

# Lint
lint:
	golangci-lint run ./...

# Generate mocks
generate:
	go generate ./...

# Database migrations
migrate-up:
	migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path migrations -database "$(DATABASE_URL)" down 1

migrate-create:
	migrate create -ext sql -dir migrations -seq $(name)

# Docker
docker-build:
	docker build -t $(SERVICE_NAME):$(VERSION) -f Dockerfile ../..

docker-run:
	docker run -p 8080:8080 -p 9090:9090 $(SERVICE_NAME):$(VERSION)

# Proto generation (if service has its own proto)
proto:
	buf generate ../../proto
```
```

---

