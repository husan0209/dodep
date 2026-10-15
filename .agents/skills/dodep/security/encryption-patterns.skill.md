#46 encryption-patterns.skill.md
Markdown

# encryption-patterns.skill.md

## РОЛЬ
Ты реализуешь шифрование данных для гемблинг-платформы.
Все персональные и финансовые данные должны быть защищены
at rest и in transit.

## КОНТЕКСТ
- KYC документы: AES-256-GCM + envelope encryption
- Персональные данные: field-level encryption
- Transit: TLS 1.3 минимум, mTLS между сервисами
- Key management: HashiCorp Vault Transit engine
- Compliance: GDPR, PCI DSS awareness

## DATA CLASSIFICATION
CRITICAL (AES-256-GCM + HSM keys):

KYC документы (паспорт, ID, selfie)
Payment credentials (если хранятся)
2FA secrets
API keys третьих сторон
SENSITIVE (AES-256-GCM + Vault managed keys):

Email, phone, full name
Date of birth
Home address
Bank account details
IP addresses (в некоторых юрисдикциях)
INTERNAL (TLS in transit, plaintext at rest):

Bet history
Session data
Game logs
Aggregated analytics
PUBLIC (no encryption needed):

Odds
Game catalog
Sports events
Promotions
text


## ENVELOPE ENCRYPTION
┌──────────────────────────────────────────────────┐
│ ENVELOPE ENCRYPTION │
│ │
│ 1. Vault генерирует Data Encryption Key (DEK) │
│ 2. DEK шифрует данные (AES-256-GCM) │
│ 3. Vault шифрует DEK своим Master Key (KEK) │
│ 4. Хранится: encrypted_data + encrypted_DEK │
│ │
│ Vault Master Key (KEK) │
│ │ │
│ ▼ │
│ Encrypted DEK ──────decrypt──────▶ DEK │
│ │ │
│ ▼ │
│ Encrypted Data ─────decrypt──────▶ Data │
│ │
│ Преимущество: ротация KEK не требует │
│ перешифровки всех данных │
└──────────────────────────────────────────────────┘

text


## RUST ENCRYPTION SERVICE

```rust
// Encryption service через Vault Transit
use aes_gcm::{Aes256Gcm, Key, Nonce};
use aes_gcm::aead::{Aead, NewAead, OsRng};
use rand::RngCore;

pub struct EncryptionService {
    vault_client: VaultClient,
    key_name: String,
}

impl EncryptionService {
    /// Envelope encryption: генерирует DEK, шифрует данные,
    /// шифрует DEK через Vault
    pub async fn encrypt(&self, plaintext: &[u8]) -> Result<EncryptedPayload> {
        // 1. Генерируем случайный DEK (256 bit)
        let mut dek = [0u8; 32];
        OsRng.fill_bytes(&mut dek);

        // 2. Генерируем nonce (96 bit)
        let mut nonce_bytes = [0u8; 12];
        OsRng.fill_bytes(&mut nonce_bytes);
        let nonce = Nonce::from_slice(&nonce_bytes);

        // 3. Шифруем данные с DEK (AES-256-GCM)
        let key = Key::from_slice(&dek);
        let cipher = Aes256Gcm::new(key);
        let ciphertext = cipher
            .encrypt(nonce, plaintext)
            .map_err(|_| Error::EncryptionFailed)?;

        // 4. Шифруем DEK через Vault Transit
        let encrypted_dek = self.vault_client
            .transit_encrypt(&self.key_name, &dek)
            .await?;

        // 5. Зануляем DEK в памяти
        dek.zeroize();

        Ok(EncryptedPayload {
            version: 1,
            algorithm: "AES-256-GCM".to_string(),
            nonce: base64::encode(&nonce_bytes),
            ciphertext: base64::encode(&ciphertext),
            encrypted_dek: encrypted_dek,
        })
    }

    /// Расшифровка: расшифровываем DEK через Vault, 
    /// затем данные через DEK
    pub async fn decrypt(&self, payload: &EncryptedPayload) -> Result<Vec<u8>> {
        // 1. Расшифровываем DEK через Vault
        let dek = self.vault_client
            .transit_decrypt(&self.key_name, &payload.encrypted_dek)
            .await?;

        // 2. Расшифровываем данные
        let nonce_bytes = base64::decode(&payload.nonce)?;
        let ciphertext = base64::decode(&payload.ciphertext)?;

        let key = Key::from_slice(&dek);
        let nonce = Nonce::from_slice(&nonce_bytes);
        let cipher = Aes256Gcm::new(key);

        let plaintext = cipher
            .decrypt(nonce, ciphertext.as_ref())
            .map_err(|_| Error::DecryptionFailed)?;

        Ok(plaintext)
    }
}

#[derive(Serialize, Deserialize)]
pub struct EncryptedPayload {
    pub version: u8,
    pub algorithm: String,
    pub nonce: String,          // base64
    pub ciphertext: String,     // base64
    pub encrypted_dek: String,  // Vault ciphertext
}
FIELD-LEVEL ENCRYPTION
Rust

// Шифрование конкретных полей в БД
#[derive(sqlx::FromRow)]
pub struct UserRecord {
    pub id: i64,
    pub email_encrypted: Vec<u8>,      // зашифровано
    pub email_hash: String,            // SHA-256 для поиска
    pub phone_encrypted: Option<Vec<u8>>,
    pub phone_hash: Option<String>,
    pub country_code: String,          // не шифруем (не PII сам по себе)
    pub status: String,                // не шифруем
    pub created_at: DateTime<Utc>,     // не шифруем
}

impl UserRecord {
    pub async fn decrypt_email(
        &self, 
        crypto: &EncryptionService,
    ) -> Result<String> {
        let payload: EncryptedPayload = 
            serde_json::from_slice(&self.email_encrypted)?;
        let bytes = crypto.decrypt(&payload).await?;
        Ok(String::from_utf8(bytes)?)
    }
}

// Поиск по зашифрованному полю через hash
pub async fn find_by_email(
    pool: &PgPool,
    crypto: &EncryptionService,
    email: &str,
) -> Result<Option<User>> {
    let email_hash = sha256_hex(email.to_lowercase().trim());

    let record = sqlx::query_as!(
        UserRecord,
        "SELECT * FROM users WHERE email_hash = $1",
        email_hash
    )
    .fetch_optional(pool)
    .await?;

    match record {
        Some(r) => {
            let decrypted_email = r.decrypt_email(crypto).await?;
            Ok(Some(User::from_record(r, decrypted_email)))
        }
        None => Ok(None),
    }
}
GO ENCRYPTION
Go

// Go — field-level encryption
package crypto

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "crypto/sha256"
    "encoding/hex"
    "io"
)

type EncryptionService struct {
    vaultClient *vault.Client
    keyName     string
}

func (s *EncryptionService) EncryptField(plaintext string) ([]byte, error) {
    // 1. Generate random DEK
    dek := make([]byte, 32)
    if _, err := io.ReadFull(rand.Reader, dek); err != nil {
        return nil, err
    }

    // 2. Encrypt with AES-256-GCM
    block, err := aes.NewCipher(dek)
    if err != nil {
        return nil, err
    }
    aesGCM, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }

    nonce := make([]byte, aesGCM.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return nil, err
    }

    ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)

    // 3. Encrypt DEK via Vault
    encryptedDEK, err := s.vaultClient.Transit().Encrypt(s.keyName, dek)
    if err != nil {
        return nil, err
    }

    // 4. Clear DEK from memory
    for i := range dek {
        dek[i] = 0
    }

    payload := EncryptedPayload{
        Version:      1,
        Ciphertext:   ciphertext,
        EncryptedDEK: encryptedDEK,
    }

    return json.Marshal(payload)
}

// Hash for searchable encrypted fields
func HashForSearch(value string) string {
    h := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(value))))
    return hex.EncodeToString(h[:])
}
KYC DOCUMENT ENCRYPTION
Rust

// KYC документы: шифрование перед загрузкой в S3
pub async fn upload_kyc_document(
    crypto: &EncryptionService,
    s3: &S3Client,
    user_id: i64,
    document_type: &str,
    file_bytes: &[u8],
) -> Result<KYCDocument> {
    // 1. Шифруем документ
    let encrypted = crypto.encrypt(file_bytes).await?;
    let encrypted_bytes = serde_json::to_vec(&encrypted)?;

    // 2. Генерируем уникальный путь
    let key = format!(
        "kyc/{}/{}_{}.enc",
        user_id,
        document_type,
        Uuid::new_v4()
    );

    // 3. Загружаем в S3 (уже зашифровано нами + SSE-S3)
    s3.put_object()
        .bucket("platform-kyc-documents")
        .key(&key)
        .body(ByteStream::from(encrypted_bytes))
        .server_side_encryption(ServerSideEncryption::Aws256)
        .content_type("application/octet-stream")
        .metadata("encryption", "envelope-aes256gcm")
        .metadata("user_id", &user_id.to_string())
        .send()
        .await?;

    Ok(KYCDocument {
        user_id,
        document_type: document_type.to_string(),
        s3_key: key,
        uploaded_at: Utc::now(),
    })
}
KEY ROTATION
Rust

// Vault Transit key rotation — не требует перешифровки данных
// Vault хранит все версии ключа и может расшифровать старыми версиями

// Периодическая ре-шифровка (опционально, для compliance)
pub async fn rewrap_encrypted_field(
    vault: &VaultClient,
    key_name: &str,
    encrypted_dek: &str,
) -> Result<String> {
    // Vault расшифрует старым ключом и зашифрует новым
    vault.transit_rewrap(key_name, encrypted_dek).await
}

// Batch rewrap для миграции
pub async fn batch_rewrap_users(
    pool: &PgPool,
    vault: &VaultClient,
) -> Result<u64> {
    let mut count = 0u64;
    let mut cursor = 0i64;

    loop {
        let users = sqlx::query!(
            r#"
            SELECT id, email_encrypted FROM users
            WHERE id > $1
            ORDER BY id
            LIMIT 1000
            "#,
            cursor
        )
        .fetch_all(pool)
        .await?;

        if users.is_empty() { break; }

        for user in &users {
            let mut payload: EncryptedPayload =
                serde_json::from_slice(&user.email_encrypted)?;

            payload.encrypted_dek = vault
                .transit_rewrap("user-data", &payload.encrypted_dek)
                .await?;

            let new_bytes = serde_json::to_vec(&payload)?;
            sqlx::query!(
                "UPDATE users SET email_encrypted = $1 WHERE id = $2",
                new_bytes,
                user.id
            )
            .execute(pool)
            .await?;

            count += 1;
        }

        cursor = users.last().unwrap().id;
    }

    Ok(count)
}
VAULT CONFIGURATION
hcl

# Vault Transit engine для шифрования
resource "vault_mount" "transit" {
  path = "transit"
  type = "transit"
}

resource "vault_transit_secret_backend_key" "user_data" {
  backend          = vault_mount.transit.path
  name             = "user-data"
  type             = "aes256-gcm96"
  deletion_allowed = false
  exportable       = false
  min_encryption_version = 1
  min_decryption_version = 1
}

resource "vault_transit_secret_backend_key" "kyc_documents" {
  backend          = vault_mount.transit.path
  name             = "kyc-documents"
  type             = "aes256-gcm96"
  deletion_allowed = false
  exportable       = false
}

resource "vault_transit_secret_backend_key" "payment_data" {
  backend          = vault_mount.transit.path
  name             = "payment-data"
  type             = "aes256-gcm96"
  deletion_allowed = false
  exportable       = false
}

# Policies: каждый сервис доступ только к своему ключу
resource "vault_policy" "user_service" {
  name   = "user-service"
  policy = <<EOT
path "transit/encrypt/user-data" {
  capabilities = ["update"]
}
path "transit/decrypt/user-data" {
  capabilities = ["update"]
}
path "transit/rewrap/user-data" {
  capabilities = ["update"]
}
EOT
}

resource "vault_policy" "kyc_service" {
  name   = "kyc-service"
  policy = <<EOT
path "transit/encrypt/kyc-documents" {
  capabilities = ["update"]
}
path "transit/decrypt/kyc-documents" {
  capabilities = ["update"]
}
EOT
}
АНТИПАТТЕРНЫ
Rust

// ❌ ПЛОХО: хардкод ключа шифрования
let key = b"my-secret-key-1234567890123456";

// ✅ ПРАВИЛЬНО: ключи из Vault

// ❌ ПЛОХО: ECB mode
let cipher = Aes256::new(key);
// ECB не скрывает паттерны в данных

// ✅ ПРАВИЛЬНО: GCM mode (authenticated encryption)
let cipher = Aes256Gcm::new(key);

// ❌ ПЛОХО: reuse nonce
let nonce = Nonce::from_slice(b"fixed-nonce!");

// ✅ ПРАВИЛЬНО: random nonce каждый раз
let mut nonce_bytes = [0u8; 12];
OsRng.fill_bytes(&mut nonce_bytes);

// ❌ ПЛОХО: не зануляем ключ после использования
let key = get_key();
encrypt(key, data);
// key остаётся в памяти

// ✅ ПРАВИЛЬНО: zeroize
use zeroize::Zeroize;
let mut key = get_key();
encrypt(&key, data);
key.zeroize();

// ❌ ПЛОХО: шифровать но не аутентифицировать
// AES-CBC без HMAC — можно модифицировать ciphertext

// ✅ ПРАВИЛЬНО: AES-GCM (шифрование + аутентификация в одном)

// ❌ ПЛОХО: SHA-256 для паролей
let hash = sha256(password);

// ✅ ПРАВИЛЬНО: Argon2id для паролей, SHA-256 только для search hashes