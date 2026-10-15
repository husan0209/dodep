package domain

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// ─── Document object storage (S3 / MinIO, SSE-KMS) ───
//
// Document BYTES live in object storage, never in Postgres or Redpanda
// events. The database keeps only FileID (the opaque upload reference);
// this file maps FileID to an S3 object reference and mints short-lived
// pre-signed GET URLs for reviewers.
//
// Encryption: buckets enforce SSE-KMS (aws:kms) with a per-environment CMK.
// Uploads carry the x-amz-server-side-encryption headers from
// UploadSSEHeaders; downloads need no extra parameters (S3 decrypts
// transparently for authorised callers). Key material (the CMK) is
// referenced by ID/alias, never embedded — and AWS credentials arrive via
// the standard env chain (AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY /
// AWS_SESSION_TOKEN, IRSA in Kubernetes), never via code.

// StorageConfig locates the KYC document bucket. Load from env.
type StorageConfig struct {
	Bucket   string
	Region   string
	Endpoint string // optional: MinIO / VPC endpoint host (no scheme); "" = AWS
	KMSKeyID string // SSE-KMS CMK id/alias/ARN; "" = bucket default encryption
	URLTTL   time.Duration
}

// StorageConfigFromEnv reads KYC_S3_BUCKET (required), KYC_S3_REGION
// (default eu-central-1), KYC_S3_ENDPOINT (optional), KYC_S3_KMS_KEY_ID
// (CMK reference, optional) and KYC_DOC_URL_TTL_SECONDS (default 900,
// clamped to [60, 604800]).
func StorageConfigFromEnv() (StorageConfig, error) {
	bucket := strings.TrimSpace(os.Getenv("KYC_S3_BUCKET"))
	if bucket == "" {
		return StorageConfig{}, NewValidationError(FieldError{Field: "kyc_s3_bucket", Message: "document bucket is not configured"})
	}
	region := strings.TrimSpace(os.Getenv("KYC_S3_REGION"))
	if region == "" {
		region = "eu-central-1"
	}
	ttl := 900
	if raw := strings.TrimSpace(os.Getenv("KYC_DOC_URL_TTL_SECONDS")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			ttl = n
		}
	}
	if ttl < 60 {
		ttl = 60
	}
	if ttl > 604800 {
		ttl = 604800
	}
	return StorageConfig{
		Bucket:   bucket,
		Region:   region,
		Endpoint: strings.TrimSpace(os.Getenv("KYC_S3_ENDPOINT")),
		KMSKeyID: strings.TrimSpace(os.Getenv("KYC_S3_KMS_KEY_ID")),
		URLTTL:   time.Duration(ttl) * time.Second,
	}, nil
}

// ObjectKey maps (user_id, file_id) to the canonical object key:
// kyc/{user_id}/{file_id}. file_id is sanitised — path separators,
// traversal and exotic characters are rejected (fail secure).
func (c StorageConfig) ObjectKey(userID int64, fileID string) (string, error) {
	if userID <= 0 {
		return "", NewValidationError(FieldError{Field: "user_id", Message: "must be positive"})
	}
	clean, err := sanitizeFileID(fileID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("kyc/%d/%s", userID, clean), nil
}

// sanitizeFileID allows [A-Za-z0-9._-] up to 255 chars, no ".." runs.
func sanitizeFileID(fileID string) (string, error) {
	clean := strings.TrimSpace(fileID)
	if clean == "" {
		return "", NewValidationError(FieldError{Field: "file_id", Message: "file reference is required"})
	}
	if len(clean) > 255 {
		return "", NewValidationError(FieldError{Field: "file_id", Message: "must be at most 255 characters"})
	}
	if strings.Contains(clean, "..") || strings.ContainsAny(clean, "/\\") {
		return "", NewValidationError(FieldError{Field: "file_id", Message: "path traversal is not allowed"})
	}
	for _, r := range clean {
		if r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-' {
			continue
		}
		return "", NewValidationError(FieldError{Field: "file_id", Message: "must match [A-Za-z0-9._-]"})
	}
	return clean, nil
}

// DocumentRef is the resolved, signable reference to one stored document.
type DocumentRef struct {
	Bucket   string
	Key      string
	Region   string
	Endpoint string
	KMSKeyID string
}

// RefFor resolves (user_id, file_id) to a DocumentRef.
func (c StorageConfig) RefFor(userID int64, fileID string) (DocumentRef, error) {
	key, err := c.ObjectKey(userID, fileID)
	if err != nil {
		return DocumentRef{}, err
	}
	return DocumentRef{
		Bucket: c.Bucket, Key: key, Region: c.Region,
		Endpoint: c.Endpoint, KMSKeyID: c.KMSKeyID,
	}, nil
}

// UploadSSEHeaders returns the headers the upload path must set so the
// object lands encrypted with SSE-KMS. Referenced by the (out-of-scope)
// upload handler; the download/presign path needs no SSE parameters.
func (c StorageConfig) UploadSSEHeaders() map[string]string {
	h := map[string]string{"x-amz-server-side-encryption": "aws:kms"}
	if c.KMSKeyID != "" {
		h["x-amz-server-side-encryption-aws-kms-key-id"] = c.KMSKeyID
	}
	return h
}

// PresignedGetURL mints a SigV4 pre-signed GET URL for one document.
// Credentials come from the standard AWS env chain; the URL itself is
// short-lived (URLTTL) and scoped to a single object key.
func (c StorageConfig) PresignedGetURL(userID int64, fileID string) (string, error) {
	ref, err := c.RefFor(userID, fileID)
	if err != nil {
		return "", err
	}
	access := strings.TrimSpace(os.Getenv("AWS_ACCESS_KEY_ID"))
	secret := strings.TrimSpace(os.Getenv("AWS_SECRET_ACCESS_KEY"))
	if access == "" || secret == "" {
		return "", NewValidationError(FieldError{Field: "aws_credentials", Message: "AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY are not configured"})
	}
	session := strings.TrimSpace(os.Getenv("AWS_SESSION_TOKEN"))
	return presignS3Get(ref, access, secret, session, int64(c.URLTTL/time.Second), time.Now().UTC()), nil
}

// presignS3Get implements AWS Signature Version 4 for S3 GET (query auth).
// Stdlib only: no SDK required, no secrets in code.
func presignS3Get(ref DocumentRef, access, secret, session string, expires int64, now time.Time) string {
	var host, canonicalURI string
	if ref.Endpoint != "" {
		host = strings.TrimPrefix(strings.TrimPrefix(ref.Endpoint, "https://"), "http://")
		canonicalURI = "/" + ref.Bucket + "/" + encodeKey(ref.Key)
	} else {
		host = ref.Bucket + ".s3." + ref.Region + ".amazonaws.com"
		canonicalURI = "/" + encodeKey(ref.Key)
	}

	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")
	credentialScope := dateStamp + "/" + ref.Region + "/s3/aws4_request"

	query := url.Values{
		"X-Amz-Algorithm":     {"AWS4-HMAC-SHA256"},
		"X-Amz-Credential":    {access + "/" + credentialScope},
		"X-Amz-Date":          {amzDate},
		"X-Amz-Expires":       {strconv.FormatInt(expires, 10)},
		"X-Amz-SignedHeaders": {"host"},
	}
	if session != "" {
		query.Set("X-Amz-Security-Token", session)
	}

	canonicalQuery := strings.ReplaceAll(query.Encode(), "+", "%20")
	canonicalHeaders := "host:" + host + "\n"
	canonicalRequest := "GET\n" + canonicalURI + "\n" + canonicalQuery + "\n" +
		canonicalHeaders + "\nhost\nUNSIGNED-PAYLOAD"
	hash := sha256.Sum256([]byte(canonicalRequest))

	stringToSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + credentialScope + "\n" + hex.EncodeToString(hash[:])
	signingKey := deriveSigningKey(secret, dateStamp, ref.Region, "s3")
	signature := hex.EncodeToString(hmacSHA256(signingKey, stringToSign))

	return "https://" + host + canonicalURI + "?" + canonicalQuery + "&X-Amz-Signature=" + signature
}

// encodeKey percent-encodes each path segment, preserving "/" separators.
func encodeKey(key string) string {
	parts := strings.Split(key, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return strings.Join(parts, "/")
}

func hmacSHA256(key []byte, data string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	return mac.Sum(nil)
}

func deriveSigningKey(secret, dateStamp, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), dateStamp)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, service)
	return hmacSHA256(kService, "aws4_request")
}
