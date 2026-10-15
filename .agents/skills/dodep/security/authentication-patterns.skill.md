## #45 authentication-patterns.skill.md

```markdown
# authentication-patterns.skill.md

## РОЛЬ
Ты реализуешь систему аутентификации для гемблинг-платформы.
JWT + Refresh Token rotation + 2FA + device tracking.

## КОНТЕКСТ
- Access Token: JWT, Ed25519 подпись, 15 минут TTL
- Refresh Token: opaque (random UUID), 7 дней, хранится в DragonflyDB
- 2FA: TOTP (Google Authenticator) + WebAuthn + SMS fallback
- Max 5 concurrent sessions per user
- Device fingerprinting для аномалий

## TOKEN ARCHITECTURE
┌─────────────────────────────────────────────────┐
│ TOKEN FLOW │
│ │
│ Login (email + password) │
│ → Verify Argon2id hash │
│ → If 2FA enabled → return temp_token │
│ → Verify 2FA code │
│ → Generate Access Token (JWT, 15m) │
│ → Generate Refresh Token (UUID, 7d) │
│ → Store refresh in DragonflyDB │
│ → Return both to client │
│ │
│ API Request │
│ → Extract JWT from Authorization header │
│ → Verify Ed25519 signature (< 1ms) │
│ → Check exp claim │
│ → Extract user_id, roles, permissions │
│ → Continue to handler │
│ │
│ Token Refresh │
│ → Client sends refresh_token │
│ → Lookup in DragonflyDB │
│ → If found → delete old, generate new pair │
│ → If NOT found → token reuse detected! │
│ → Revoke ALL tokens for this user │
│ → Force re-login │
│ │
│ Logout │
│ → Delete refresh_token from DragonflyDB │
│ → Add access_token jti to blacklist (TTL=15m) │
└─────────────────────────────────────────────────┘

text


## JWT STRUCTURE

```rust
// Rust — JWT claims
#[derive(Serialize, Deserialize)]
pub struct Claims {
    pub sub: i64,              // user_id
    pub jti: String,           // unique token id (UUID)
    pub roles: Vec<String>,    // ["player", "vip"]
    pub permissions: Vec<String>,
    pub device_id: String,     // device fingerprint
    pub iat: i64,              // issued at (unix timestamp)
    pub exp: i64,              // expires at
}

// Ed25519 signing
use ed25519_dalek::{SigningKey, VerifyingKey, Signer, Verifier};

pub struct JwtService {
    signing_key: SigningKey,
    verifying_key: VerifyingKey,
}

impl JwtService {
    pub fn generate(&self, user: &User, device_id: &str) -> Result<String> {
        let now = Utc::now().timestamp();
        let claims = Claims {
            sub: user.id,
            jti: Uuid::new_v4().to_string(),
            roles: user.roles.clone(),
            permissions: user.permissions.clone(),
            device_id: device_id.to_string(),
            iat: now,
            exp: now + 900,  // 15 minutes
        };
        
        let header = base64url_encode(r#"{"alg":"EdDSA","typ":"JWT"}"#);
        let payload = base64url_encode(&serde_json::to_string(&claims)?);
        let message = format!("{}.{}", header, payload);
        let signature = self.signing_key.sign(message.as_bytes());
        
        Ok(format!("{}.{}", message, base64url_encode(&signature.to_bytes())))
    }
    
    pub fn verify(&self, token: &str) -> Result<Claims> {
        let parts: Vec<&str> = token.split('.').collect();
        if parts.len() != 3 {
            return Err(Error::InvalidToken);
        }
        
        let message = format!("{}.{}", parts[0], parts[1]);
        let signature_bytes = base64url_decode(parts[2])?;
        let signature = ed25519_dalek::Signature::from_bytes(&signature_bytes);
        
        self.verifying_key
            .verify(message.as_bytes(), &signature)
            .map_err(|_| Error::InvalidSignature)?;
        
        let claims: Claims = serde_json::from_slice(&base64url_decode(parts[1])?)?;
        
        if claims.exp < Utc::now().timestamp() {
            return Err(Error::TokenExpired);
        }
        
        Ok(claims)
    }
}
PASSWORD HASHING
Rust

// Argon2id — единственный допустимый алгоритм
use argon2::{
    password_hash::{SaltString, PasswordHasher, PasswordVerifier},
    Argon2, Algorithm, Version, Params,
};

pub struct PasswordService;

impl PasswordService {
    fn argon2() -> Argon2<'static> {
        Argon2::new(
            Algorithm::Argon2id,
            Version::V0x13,
            Params::new(
                65536,  // 64 MB memory
                3,      // 3 iterations
                4,      // 4 parallelism
                Some(32), // 32 byte output
            ).unwrap(),
        )
    }
    
    pub fn hash(password: &str) -> Result<String> {
        let salt = SaltString::generate(&mut OsRng);
        let hash = Self::argon2()
            .hash_password(password.as_bytes(), &salt)
            .map_err(|_| Error::HashingFailed)?;
        Ok(hash.to_string())
    }
    
    pub fn verify(password: &str, hash: &str) -> Result<bool> {
        let parsed_hash = argon2::PasswordHash::new(hash)
            .map_err(|_| Error::InvalidHash)?;
        Ok(Self::argon2()
            .verify_password(password.as_bytes(), &parsed_hash)
            .is_ok())
    }
}
REFRESH TOKEN ROTATION
Rust

// Refresh token management
pub struct RefreshTokenService {
    cache: RedisPool,
}

impl RefreshTokenService {
    // Хранение: refresh_token → {user_id, device_id, family_id, created_at}
    
    pub async fn create(
        &self,
        user_id: i64,
        device_id: &str,
    ) -> Result<String> {
        let token = Uuid::new_v4().to_string();
        let family_id = Uuid::new_v4().to_string();
        
        let key = format!("session:refresh:{token}");
        let data = serde_json::to_string(&RefreshTokenData {
            user_id,
            device_id: device_id.to_string(),
            family_id: family_id.clone(),
            created_at: Utc::now(),
        })?;
        
        self.cache.set(&key, data, Some(7 * 24 * 3600), None, false).await?;
        
        // Track family
        let family_key = format!("session:family:{family_id}");
        self.cache.sadd(&family_key, &token).await?;
        self.cache.expire(&family_key, 7 * 24 * 3600).await?;
        
        Ok(token)
    }
    
    pub async fn rotate(&self, old_token: &str) -> Result<(String, i64)> {
        let key = format!("session:refresh:{old_token}");
        
        // Atomic get + delete
        let data_str: Option<String> = self.cache.getdel(&key).await?;
        
        let data: RefreshTokenData = match data_str {
            Some(s) => serde_json::from_str(&s)?,
            None => {
                // TOKEN REUSE DETECTED!
                // Кто-то использовал уже использованный токен
                // Это может быть атака — отзываем ВСЕ токены семьи
                tracing::warn!(
                    token = old_token,
                    "Refresh token reuse detected! Revoking all sessions."
                );
                // Тут нужно найти family_id и отозвать всё
                return Err(Error::TokenReused);
            }
        };
        
        // Создать новый токен в той же семье
        let new_token = Uuid::new_v4().to_string();
        let new_key = format!("session:refresh:{new_token}");
        let new_data = serde_json::to_string(&RefreshTokenData {
            user_id: data.user_id,
            device_id: data.device_id,
            family_id: data.family_id.clone(),
            created_at: Utc::now(),
        })?;
        
        self.cache.set(&new_key, new_data, Some(7 * 24 * 3600), None, false).await?;
        
        // Update family
        let family_key = format!("session:family:{}", data.family_id);
        self.cache.srem(&family_key, old_token).await?;
        self.cache.sadd(&family_key, &new_token).await?;
        
        Ok((new_token, data.user_id))
    }
    
    pub async fn revoke_all(&self, user_id: i64) -> Result<()> {
        // Найти все сессии пользователя
        let sessions_key = format!("session:{user_id}:tokens");
        let tokens: Vec<String> = self.cache.smembers(&sessions_key).await?;
        
        for token in tokens {
            let key = format!("session:refresh:{token}");
            self.cache.del(&key).await?;
        }
        
        self.cache.del(&sessions_key).await?;
        Ok(())
    }
}
2FA — TOTP
Rust

// TOTP implementation
use totp_rs::{TOTP, Algorithm, Secret};

pub struct TwoFactorService;

impl TwoFactorService {
    pub fn generate_secret(email: &str) -> Result<(String, String)> {
        let secret = Secret::generate_secret();
        let totp = TOTP::new(
            Algorithm::SHA1,
            6,        // digits
            1,        // skew (допуск ±1 интервал)
            30,       // period (30 секунд)
            secret.to_bytes().unwrap(),
            Some("GamblingPlatform".to_string()),
            email.to_string(),
        )?;
        
        let secret_base32 = secret.to_encoded().to_string();
        let qr_url = totp.get_url();
        
        Ok((secret_base32, qr_url))
    }
    
    pub fn verify_code(secret_base32: &str, code: &str) -> Result<bool> {
        let secret = Secret::Encoded(secret_base32.to_string());
        let totp = TOTP::new(
            Algorithm::SHA1,
            6,
            1,
            30,
            secret.to_bytes().unwrap(),
            None,
            String::new(),
        )?;
        
        Ok(totp.check_current(code)?)
    }
}
LOGIN FLOW — COMPLETE
Rust

pub async fn login(
    state: &AppState,
    req: LoginRequest,
    ip: IpAddr,
    user_agent: &str,
    device_fp: &str,
) -> Result<LoginResponse> {
    // 1. Find user (constant-time: same response for exists/not-exists)
    let user = state.user_repo.find_by_email(&req.email).await?;
    
    // 2. Verify password
    let user = match user {
        Some(u) => {
            if !PasswordService::verify(&req.password, &u.password_hash)? {
                // Track failed attempt
                state.login_tracker.record_failure(ip, &req.email).await;
                return Err(Error::InvalidCredentials);
            }
            u
        }
        None => {
            // Dummy hash to prevent timing attack
            let _ = PasswordService::verify(&req.password, DUMMY_HASH);
            state.login_tracker.record_failure(ip, &req.email).await;
            return Err(Error::InvalidCredentials);
        }
    };
    
    // 3. Check account status
    match user.status {
        UserStatus::Blocked => return Err(Error::AccountBlocked),
        UserStatus::SelfExcluded => return Err(Error::SelfExcluded),
        UserStatus::Pending => return Err(Error::EmailNotVerified),
        UserStatus::Active => {}
        _ => return Err(Error::AccountInactive),
    }
    
    // 4. Check login attempts (brute force protection)
    if state.login_tracker.is_locked(ip, &req.email).await? {
        return Err(Error::TooManyAttempts { retry_after: 900 });
    }
    
    // 5. Check 2FA
    if user.two_fa_enabled {
        let temp_token = state.jwt.generate_temp(&user)?;
        return Ok(LoginResponse::Requires2FA { temp_token });
    }
    
    // 6. Generate tokens
    let access_token = state.jwt.generate(&user, device_fp)?;
    let refresh_token = state.refresh_service
        .create(user.id, device_fp).await?;
    
    // 7. Track session
    state.session_service
        .create(user.id, device_fp, ip, user_agent).await?;
    
    // 8. Check for anomalies
    if state.anomaly_detector.is_suspicious(user.id, ip, device_fp).await? {
        // Не блокировать, но уведомить
        state.notification_service
            .send_suspicious_login(user.id, ip, device_fp).await?;
    }
    
    // 9. Reset failed attempts
    state.login_tracker.reset(ip, &req.email).await;
    
    // 10. Update last login
    state.user_repo.update_last_login(user.id, ip).await?;
    
    // 11. Publish event
    state.events.publish_user_logged_in(user.id, ip, device_fp).await?;
    
    Ok(LoginResponse::Success {
        access_token,
        refresh_token,
        user: UserProfile::from(user),
    })
}
AUTH MIDDLEWARE
Rust

// Rust — Axum middleware
pub async fn auth_middleware(
    State(state): State<AppState>,
    mut request: Request,
    next: Next,
) -> Result<Response, AppError> {
    let token = request
        .headers()
        .get(AUTHORIZATION)
        .and_then(|v| v.to_str().ok())
        .and_then(|v| v.strip_prefix("Bearer "))
        .ok_or(AppError::Unauthorized("Missing token".into()))?;
    
    // Verify JWT (< 1ms, no network call)
    let claims = state.jwt.verify(token)
        .map_err(|e| match e {
            Error::TokenExpired => AppError::Unauthorized("Token expired".into()),
            _ => AppError::Unauthorized("Invalid token".into()),
        })?;
    
    // Check blacklist (revoked tokens)
    let blacklist_key = format!("token:blacklist:{}", claims.jti);
    if state.cache.exists(&blacklist_key).await? {
        return Err(AppError::Unauthorized("Token revoked".into()));
    }
    
    let auth_user = AuthUser {
        id: claims.sub,
        roles: claims.roles,
        permissions: claims.permissions,
        device_id: claims.device_id,
    };
    
    request.extensions_mut().insert(auth_user);
    Ok(next.run(request).await)
}
Go

// Go — Fiber middleware
func AuthMiddleware(jwtService *JWTService, cache *redis.Client) fiber.Handler {
    return func(c *fiber.Ctx) error {
        token := c.Get("Authorization")
        if !strings.HasPrefix(token, "Bearer ") {
            return c.Status(401).JSON(ErrorResponse{Code: "AUTH_MISSING_TOKEN"})
        }
        token = strings.TrimPrefix(token, "Bearer ")
        
        claims, err := jwtService.Verify(token)
        if err != nil {
            return c.Status(401).JSON(ErrorResponse{Code: "AUTH_INVALID_TOKEN"})
        }
        
        // Check blacklist
        blacklisted, _ := cache.Exists(c.Context(), 
            fmt.Sprintf("token:blacklist:%s", claims.JTI)).Result()
        if blacklisted > 0 {
            return c.Status(401).JSON(ErrorResponse{Code: "AUTH_TOKEN_REVOKED"})
        }
        
        c.Locals("user_id", claims.Sub)
        c.Locals("roles", claims.Roles)
        c.Locals("permissions", claims.Permissions)
        
        return c.Next()
    }
}
PERMISSION CHECK
Rust

// Rust — permission check middleware/extractor
pub struct RequirePermission(pub &'static str);

#[async_trait]
impl<S> FromRequestParts<S> for RequirePermission
where
    S: Send + Sync,
{
    type Rejection = AppError;

    async fn from_request_parts(parts: &mut Parts, _state: &S) -> Result<Self, Self::Rejection> {
        let user = parts.extensions.get::<AuthUser>()
            .ok_or(AppError::Unauthorized("No auth context".into()))?;
        
        // "all.*" = superadmin
        if !user.permissions.contains(&"all.*".to_string())
            && !user.permissions.contains(&self.0.to_string())
        {
            return Err(AppError::Forbidden(format!(
                "Missing permission: {}", self.0
            )));
        }
        
        Ok(Self(self.0))
    }
}

// Использование в handler:
async fn void_bet(
    _perm: RequirePermission("bet.void"),
    Extension(user): Extension<AuthUser>,
    Path(bet_id): Path<i64>,
) -> Result<Json<Response>> {
    // Только пользователи с permission "bet.void"
}
АНТИПАТТЕРНЫ
Rust

// ❌ ПЛОХО: хранить access token в localStorage
// XSS может украсть его
localStorage.setItem('access_token', token);

// ✅ ПРАВИЛЬНО: access token только в памяти (JS variable)
// Refresh token в httpOnly cookie или localStorage (менее критичен)

// ❌ ПЛОХО: JWT без expiry
Claims { sub: user_id }  // нет exp

// ✅ ПРАВИЛЬНО: всегда exp
Claims { sub: user_id, exp: now + 900 }

// ❌ ПЛОХО: symmetric signing (HMAC) с shared secret
// Любой сервис с ключом может создавать токены

// ✅ ПРАВИЛЬНО: asymmetric (Ed25519)
// Только auth-service имеет private key
// Все сервисы имеют public key для верификации

// ❌ ПЛОХО: не отзывать токены при смене пароля
// Старые сессии продолжают работать

// ✅ ПРАВИЛЬНО: revoke_all при:
//   - Смене пароля
//   - Включении/выключении 2FA
//   - Блокировке аккаунта
//   - Self-exclusion
//   - Обнаружении компрометации

// ❌ ПЛОХО: различать "user not found" и "wrong password"
if user.is_none() { return "User not found" }
if !verify_password() { return "Wrong password" }

// ✅ ПРАВИЛЬНО: единый ответ
return "Invalid credentials"
// + dummy hash check для constant time