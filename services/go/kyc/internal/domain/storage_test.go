package domain

import (
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testStorageConfig() StorageConfig {
	return StorageConfig{Bucket: "kyc-docs", Region: "eu-central-1", URLTTL: 900000000000}
}

func TestStorageConfigFromEnv(t *testing.T) {
	t.Setenv("KYC_S3_BUCKET", "prod-kyc-docs")
	t.Setenv("KYC_S3_REGION", "us-east-1")
	t.Setenv("KYC_S3_KMS_KEY_ID", "alias/kyc-docs")
	t.Setenv("KYC_DOC_URL_TTL_SECONDS", "600")
	t.Setenv("KYC_S3_ENDPOINT", "")

	cfg, err := StorageConfigFromEnv()
	require.NoError(t, err)
	assert.Equal(t, "prod-kyc-docs", cfg.Bucket)
	assert.Equal(t, "us-east-1", cfg.Region)
	assert.Equal(t, "alias/kyc-docs", cfg.KMSKeyID)

	t.Setenv("KYC_S3_BUCKET", "")
	_, err = StorageConfigFromEnv()
	require.Error(t, err, "missing bucket must fail secure")
}

func TestObjectKey(t *testing.T) {
	t.Parallel()
	cfg := testStorageConfig()

	key, err := cfg.ObjectKey(42, "scan-front.png")
	require.NoError(t, err)
	assert.Equal(t, "kyc/42/scan-front.png", key)

	for _, bad := range []string{"", "  ", "../secret", "a/b", `a\b`, "x$evil", strings.Repeat("a", 256)} {
		_, err := cfg.ObjectKey(42, bad)
		assert.Error(t, err, "file_id %q must be rejected", bad)
	}
	_, err = cfg.ObjectKey(0, "ok.png")
	assert.Error(t, err, "zero user must be rejected")
}

func TestUploadSSEHeaders(t *testing.T) {
	t.Parallel()
	withKMS := StorageConfig{KMSKeyID: "arn:aws:kms:x:key/1"}
	h := withKMS.UploadSSEHeaders()
	assert.Equal(t, "aws:kms", h["x-amz-server-side-encryption"])
	assert.Equal(t, "arn:aws:kms:x:key/1", h["x-amz-server-side-encryption-aws-kms-key-id"])

	plain := StorageConfig{}
	h = plain.UploadSSEHeaders()
	assert.Equal(t, "aws:kms", h["x-amz-server-side-encryption"])
	_, hasKey := h["x-amz-server-side-encryption-aws-kms-key-id"]
	assert.False(t, hasKey)
}

func TestPresignedGetURL_Structure(t *testing.T) {
	t.Setenv("KYC_S3_BUCKET", "kyc-docs")
	t.Setenv("KYC_S3_REGION", "eu-central-1")
	t.Setenv("KYC_S3_ENDPOINT", "")
	t.Setenv("KYC_DOC_URL_TTL_SECONDS", "900")
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIATESTKEY")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret")
	t.Setenv("AWS_SESSION_TOKEN", "")

	cfg, err := StorageConfigFromEnv()
	require.NoError(t, err)
	raw, err := cfg.PresignedGetURL(7, "doc-1.png")
	require.NoError(t, err)

	u, err := url.Parse(raw)
	require.NoError(t, err)
	assert.Equal(t, "https", u.Scheme, "presigned URLs must never use http")
	assert.Equal(t, "kyc-docs.s3.eu-central-1.amazonaws.com", u.Host)
	assert.Equal(t, "/kyc/7/doc-1.png", u.Path)
	q := u.Query()
	assert.Equal(t, "AWS4-HMAC-SHA256", q.Get("X-Amz-Algorithm"))
	assert.Equal(t, "900", q.Get("X-Amz-Expires"))
	assert.Contains(t, q.Get("X-Amz-Credential"), "AKIATESTKEY")
	assert.Len(t, q.Get("X-Amz-Signature"), 64, "signature must be hex sha256")
	assert.NotContains(t, raw, "test-secret", "secret must never appear in the URL")
}

func TestPresignedGetURL_NoCredsFailsSecure(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	cfg := testStorageConfig()
	_, err := cfg.PresignedGetURL(1, "a.png")
	require.Error(t, err)
}
