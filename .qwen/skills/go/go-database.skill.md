SKILL #18 — go-database.skill.md
Markdown

# go-database.skill.md
# GAMBLING PLATFORM — GO DATABASE PATTERNS
# Version: 1.0.0 | Format: B (400-600 lines)
# Loaded by: Go Business Agent

# ============================================================
# SECTION 1: CONTEXT
# ============================================================

Go services use GORM for PostgreSQL access.
Repository pattern: service depends on interface, not GORM directly.
GORM models are SEPARATE from domain entities — convert at boundary.

Connection pooling: GORM built-in + PgBouncer in front.

# ============================================================
# SECTION 2: CONNECTION SETUP
# ============================================================

```go
package database

import (
    "fmt"
    "time"

    "github.com/rs/zerolog/log"
    "gorm.io/driver/postgres"
    "gorm.io/gorm"
    "gorm.io/gorm/logger"
)

type Config struct {
    Host            string `mapstructure:"host"`
    Port            int    `mapstructure:"port"`
    Database        string `mapstructure:"database"`
    Username        string `mapstructure:"username"`
    Password        string `mapstructure:"password"`
    MaxOpenConns    int    `mapstructure:"max_open_conns"`    // 30
    MaxIdleConns    int    `mapstructure:"max_idle_conns"`    // 10
    ConnMaxLifetime int    `mapstructure:"conn_max_lifetime"` // 1800 (seconds)
    SSLMode         string `mapstructure:"ssl_mode"`          // "require"
}

func NewPool(cfg Config) (*gorm.DB, error) {
    dsn := fmt.Sprintf(
        "host=%s port=%d user=%s password=%s dbname=%s sslmode=%s application_name=%s",
        cfg.Host, cfg.Port, cfg.Username, cfg.Password, cfg.Database, cfg.SSLMode, "auth-service",
    )

    db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
        Logger:                 newZerologGormLogger(),
        SkipDefaultTransaction: true,  // manual transactions, not auto-wrap
        PrepareStmt:            true,  // prepared statement cache
    })
    if err != nil {
        return nil, fmt.Errorf("connect database: %w", err)
    }

    sqlDB, err := db.DB()
    if err != nil {
        return nil, fmt.Errorf("get sql.DB: %w", err)
    }

    sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
    sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
    sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)

    // Verify connection
    if err := sqlDB.Ping(); err != nil {
        return nil, fmt.Errorf("ping database: %w", err)
    }

    log.Info().Str("host", cfg.Host).Int("port", cfg.Port).Msg("Database connected")
    return db, nil
}

func Close(db *gorm.DB) {
    sqlDB, _ := db.DB()
    if sqlDB != nil {
        sqlDB.Close()
    }
}
============================================================
SECTION 3: GORM MODELS vs DOMAIN ENTITIES
============================================================
Go

// ── GORM Model (internal/repository/models.go) ──
// Only used in repository layer. NEVER exported to service.

type userModel struct {
    ID            int64      `gorm:"primaryKey;autoIncrement"`
    UUID          string     `gorm:"type:uuid;uniqueIndex;not null"`
    Email         string     `gorm:"type:varchar(255);uniqueIndex;not null"`
    Phone         *string    `gorm:"type:varchar(20);uniqueIndex"`
    PasswordHash  string     `gorm:"type:varchar(255);not null"`
    Status        string     `gorm:"type:user_status;not null;default:'pending'"`
    KYCLevel      int        `gorm:"type:smallint;not null;default:0"`
    CountryCode   string     `gorm:"type:char(2);not null"`
    CurrencyCode  string     `gorm:"type:char(3);not null"`
    LoginAttempts int        `gorm:"type:int;not null;default:0"`
    CreatedAt     time.Time  `gorm:"autoCreateTime"`
    UpdatedAt     time.Time  `gorm:"autoUpdateTime"`
    LastLoginAt   *time.Time
}

func (userModel) TableName() string { return "users" }

// ── Conversion methods ──

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

func userModelFromDomain(u *domain.User) *userModel {
    return &userModel{
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

// ── Domain Entity (internal/domain/user.go) ──
// Pure Go struct with NO gorm tags, NO external dependencies

type User struct {
    ID           int64
    UUID         uuid.UUID
    Email        string
    Phone        *string
    PasswordHash string
    Status       UserStatus
    KYCLevel     int
    CountryCode  string
    CurrencyCode string
    CreatedAt    time.Time
    UpdatedAt    time.Time
    LastLoginAt  *time.Time
}
============================================================
SECTION 4: REPOSITORY IMPLEMENTATION
============================================================
Go

// internal/repository/user_repo.go

type userRepository struct {
    db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
    return &userRepository{db: db}
}

func (r *userRepository) Create(ctx context.Context, user *domain.User) error {
    model := userModelFromDomain(user)
    
    result := r.db.WithContext(ctx).Create(model)
    if result.Error != nil {
        return r.mapError(result.Error)
    }
    
    // Set generated fields back
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
            return nil, nil // not found → nil, nil (not error)
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

func (r *userRepository) Update(ctx context.Context, user *domain.User) error {
    result := r.db.WithContext(ctx).
        Model(&userModel{}).
        Where("id = ?", user.ID).
        Updates(map[string]interface{}{
            "email":         user.Email,
            "phone":         user.Phone,
            "kyc_level":     user.KYCLevel,
            "country_code":  user.CountryCode,
            "currency_code": user.CurrencyCode,
            "updated_at":    time.Now(),
        })
    
    if result.Error != nil {
        return r.mapError(result.Error)
    }
    if result.RowsAffected == 0 {
        return domain.ErrUserNotFound
    }
    return nil
}

// State transition with WHERE clause guard
func (r *userRepository) UpdateStatus(ctx context.Context, id int64, from, to domain.UserStatus) error {
    result := r.db.WithContext(ctx).
        Model(&userModel{}).
        Where("id = ? AND status = ?", id, string(from)).
        Update("status", string(to))
    
    if result.Error != nil {
        return fmt.Errorf("update status: %w", result.Error)
    }
    if result.RowsAffected == 0 {
        return domain.ErrConflict // already changed by another process
    }
    return nil
}

// Pagination with cursor
func (r *userRepository) List(ctx context.Context, params ListParams) ([]*domain.User, string, error) {
    query := r.db.WithContext(ctx).Model(&userModel{})
    
    // Apply cursor
    if params.Cursor != "" {
        cursorID, err := decodeCursor(params.Cursor)
        if err == nil {
            query = query.Where("id < ?", cursorID)
        }
    }
    
    // Apply filters
    if params.Status != "" {
        query = query.Where("status = ?", params.Status)
    }
    if params.Country != "" {
        query = query.Where("country_code = ?", params.Country)
    }
    
    var models []userModel
    result := query.
        Order("id DESC").
        Limit(params.PageSize + 1). // fetch one extra to check has_more
        Find(&models)
    
    if result.Error != nil {
        return nil, "", fmt.Errorf("list users: %w", result.Error)
    }
    
    hasMore := len(models) > params.PageSize
    if hasMore {
        models = models[:params.PageSize]
    }
    
    users := make([]*domain.User, len(models))
    for i, m := range models {
        m := m
        users[i] = m.toDomain()
    }
    
    var nextCursor string
    if hasMore && len(models) > 0 {
        nextCursor = encodeCursor(models[len(models)-1].ID)
    }
    
    return users, nextCursor, nil
}

// Error mapping
func (r *userRepository) mapError(err error) error {
    var pgErr *pgconn.PgError
    if errors.As(err, &pgErr) {
        switch pgErr.Code {
        case "23505": // unique_violation
            if strings.Contains(pgErr.ConstraintName, "email") {
                return domain.ErrEmailExists
            }
            if strings.Contains(pgErr.ConstraintName, "phone") {
                return domain.ErrPhoneExists
            }
            return domain.ErrConflict
        case "23514": // check_violation
            return domain.NewValidationError(
                domain.FieldError{Field: "unknown", Message: "Constraint violation"})
        }
    }
    return fmt.Errorf("database error: %w", err)
}
============================================================
SECTION 5: TRANSACTIONS
============================================================
Go

// ── Using GORM transactions ──

func (r *paymentRepository) CreateWithLedger(
    ctx context.Context,
    payment *domain.Payment,
    entries []*domain.LedgerEntry,
) error {
    return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
        // Insert payment
        model := paymentModelFromDomain(payment)
        if err := tx.Create(model).Error; err != nil {
            return fmt.Errorf("create payment: %w", err)
        }
        payment.ID = model.ID
        
        // Insert ledger entries
        for _, entry := range entries {
            entryModel := ledgerModelFromDomain(entry)
            entryModel.ReferenceID = payment.ID
            if err := tx.Create(entryModel).Error; err != nil {
                return fmt.Errorf("create ledger entry: %w", err)
            }
        }
        
        return nil // commit
    })
    // If returned error → auto rollback
}

// ── Transaction with optimistic locking ──

func (r *walletRepository) DebitWithVersion(
    ctx context.Context,
    userID int64,
    amount decimal.Decimal,
    expectedVersion int64,
) error {
    result := r.db.WithContext(ctx).
        Model(&walletModel{}).
        Where("user_id = ? AND version = ? AND balance >= ?",
            userID, expectedVersion, amount).
        Updates(map[string]interface{}{
            "balance": gorm.Expr("balance - ?", amount),
            "version": gorm.Expr("version + 1"),
        })
    
    if result.Error != nil {
        return fmt.Errorf("debit wallet: %w", result.Error)
    }
    if result.RowsAffected == 0 {
        return domain.ErrConcurrencyConflict
    }
    return nil
}
============================================================
SECTION 6: RAW SQL (for complex queries)
============================================================
Go

// When GORM query builder is insufficient, use raw SQL

func (r *reportRepository) GetDailyRevenue(
    ctx context.Context,
    dateFrom, dateTo time.Time,
) ([]DailyRevenue, error) {
    var results []DailyRevenue
    
    err := r.db.WithContext(ctx).Raw(`
        SELECT
            DATE(created_at) as date,
            currency_code,
            SUM(CASE WHEN type = 'bet_loss' THEN amount ELSE 0 END) as ggr,
            SUM(CASE WHEN type = 'deposit' THEN amount ELSE 0 END) as deposits,
            SUM(CASE WHEN type = 'withdrawal' THEN amount ELSE 0 END) as withdrawals,
            COUNT(DISTINCT user_id) as unique_users
        FROM transactions
        WHERE created_at BETWEEN ? AND ?
        GROUP BY DATE(created_at), currency_code
        ORDER BY date DESC
    `, dateFrom, dateTo).Scan(&results).Error
    
    return results, err
}

// ── Batch operations ──

func (r *betRepository) BatchUpdateStatus(
    ctx context.Context,
    betIDs []int64,
    fromStatus, toStatus string,
) (int64, error) {
    result := r.db.WithContext(ctx).
        Model(&betModel{}).
        Where("id IN ? AND status = ?", betIDs, fromStatus).
        Update("status", toStatus)
    
    return result.RowsAffected, result.Error
}
============================================================
SECTION 7: CURSOR PAGINATION HELPERS
============================================================
Go

import "encoding/base64"

func encodeCursor(id int64) string {
    return base64.StdEncoding.EncodeToString(
        []byte(fmt.Sprintf("%d", id)),
    )
}

func decodeCursor(cursor string) (int64, error) {
    bytes, err := base64.StdEncoding.DecodeString(cursor)
    if err != nil {
        return 0, err
    }
    return strconv.ParseInt(string(bytes), 10, 64)
}
============================================================
SECTION 8: ANTI-PATTERNS
============================================================
text

❌ NEVER use GORM models in service/handler (convert at repo boundary)
❌ NEVER use db.Save() — it updates ALL fields, even zero values
   ✅ Use db.Updates() with explicit map or selected fields
❌ NEVER use db.Delete() for user data — use soft delete (deleted_at)
❌ NEVER return gorm.ErrRecordNotFound to service — return nil, nil
❌ NEVER use OFFSET for paginated APIs — use cursor-based
❌ NEVER skip WithContext(ctx) — breaks tracing and cancellation
❌ NEVER use string concatenation in WHERE — always parameterized
❌ NEVER fetch all columns when only ID needed — use Select()
❌ NEVER do N+1 queries in loop — use Preload or IN clause
❌ NEVER skip RowsAffected check after UPDATE/DELETE
❌ NEVER store money as float64 in model — use decimal or string