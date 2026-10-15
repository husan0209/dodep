package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/opus-casino/kyc/internal/cache"
	"github.com/opus-casino/kyc/internal/crypto"
	"github.com/opus-casino/kyc/internal/domain"
	"github.com/opus-casino/kyc/internal/screening"
	"github.com/opus-casino/kyc/internal/testutil"
)

// ─── Harness (fake repo, memory cache, stub screening) ───

type harness struct {
	svc  *KYCService
	repo *testutil.FakeRepo
}

func newHarness() *harness {
	repo := testutil.NewFakeRepo()
	cr, err := crypto.Ephemeral()
	if err != nil {
		panic(err)
	}
	svc := NewKYCService(repo, cache.NewMemory(), screening.StubProvider{}, cr,
		Config{WebhookSecret: "test-secret"}, zap.NewNop())
	return &harness{svc: svc, repo: repo}
}

func future(d int) *time.Time {
	t := time.Now().AddDate(0, 0, d)
	return &t
}

func submitPassport(t *testing.T, h *harness, userID int64, number string) *domain.Document {
	t.Helper()
	doc, err := h.svc.SubmitDocument(context.Background(), SubmitInput{
		UserID: userID, Type: domain.DocPassport, Side: domain.SideFront,
		FileID: "file-" + number, DocumentNumber: number,
		IssuingCountry: "DE", ExpiresAt: future(365),
	})
	require.NoError(t, err)
	require.Equal(t, domain.DocSubmitted, doc.Status)
	return doc
}

func submitSelfie(t *testing.T, h *harness, userID int64) *domain.Document {
	t.Helper()
	doc, err := h.svc.SubmitDocument(context.Background(), SubmitInput{
		UserID: userID, Type: domain.DocSelfie, Side: domain.SideSingle,
		FileID: "selfie-file",
	})
	require.NoError(t, err)
	return doc
}

func verifyDoc(t *testing.T, h *harness, id uuid.UUID, name string) {
	t.Helper()
	_, err := h.svc.VerifyDocument(context.Background(), VerifyInput{
		DocumentID: id, VerifiedBy: "admin-1", FullName: name,
	})
	require.NoError(t, err)
}

// ─── Tests ───

func TestSubmitDocument_ValidAdvancesToPending(t *testing.T) {
	h := newHarness()
	doc := submitPassport(t, h, 1, "P100")

	st, err := h.svc.GetStatus(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusPending, st.Record.Status)
	assert.Equal(t, domain.LevelNone, st.Record.Level)
	assert.NotEqual(t, "P100", h.repo.Docs[doc.ID].DocumentNumber, "PII must be encrypted at rest")
	require.NotEmpty(t, h.repo.Audits)
}

func TestSubmitDocument_ExpiredRejected(t *testing.T) {
	h := newHarness()
	past := time.Now().Add(-time.Hour)
	_, err := h.svc.SubmitDocument(context.Background(), SubmitInput{
		UserID: 1, Type: domain.DocPassport, Side: domain.SideFront,
		FileID: "f", DocumentNumber: "X1", IssuingCountry: "DE", ExpiresAt: &past,
	})
	require.Error(t, err)
	var ve *domain.ValidationError
	require.ErrorAs(t, err, &ve)
}

func TestSubmitDocument_DuplicateNumberAcrossUsers(t *testing.T) {
	h := newHarness()
	submitPassport(t, h, 1, "SHARED99")
	_, err := h.svc.SubmitDocument(context.Background(), SubmitInput{
		UserID: 2, Type: domain.DocPassport, Side: domain.SideFront,
		FileID: "f2", DocumentNumber: "shared99", // case-insensitive match
		IssuingCountry: "DE", ExpiresAt: future(30),
	})
	assert.ErrorIs(t, err, domain.ErrDocumentAlreadyUsed)
}

func TestVerifyFlow_IdentityUpgrade(t *testing.T) {
	h := newHarness()
	p := submitPassport(t, h, 7, "ID700")
	s := submitSelfie(t, h, 7)
	verifyDoc(t, h, p.ID, "John Smith")
	verifyDoc(t, h, s.ID, "John Smith")

	st, err := h.svc.GetStatus(context.Background(), 7)
	require.NoError(t, err)
	assert.Equal(t, domain.LevelIdentity, st.Record.Level)
	assert.Equal(t, domain.StatusVerified, st.Record.Status)
	assert.Equal(t, domain.ScreeningClear, st.Record.ScreeningStatus)
	require.NotNil(t, st.Record.VerifiedAt)
}

func TestRejectFlow_StatusRejectedWhenNothingPending(t *testing.T) {
	h := newHarness()
	p := submitPassport(t, h, 8, "ID800")
	_, err := h.svc.RejectDocument(context.Background(), RejectInput{
		DocumentID: p.ID, RejectedBy: "admin-1",
		Reason: "blurry image", RequiredActions: []string{"re-upload front side"},
	})
	require.NoError(t, err)

	st, err := h.svc.GetStatus(context.Background(), 8)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusRejected, st.Record.Status)
	assert.Equal(t, "blurry image", st.Record.RejectionReason)
}

func TestRequestUpgrade_StepwiseEnforced(t *testing.T) {
	h := newHarness()
	_, err := h.svc.RequestUpgrade(context.Background(), 9, domain.LevelEnhanced)
	assert.ErrorIs(t, err, domain.ErrInvalidLevelTransition)

	missing, err := h.svc.RequestUpgrade(context.Background(), 9, domain.LevelBasic)
	require.NoError(t, err)
	assert.Empty(t, missing)
	st, err := h.svc.GetStatus(context.Background(), 9)
	require.NoError(t, err)
	assert.Equal(t, domain.LevelBasic, st.Record.Level)

	missing, err = h.svc.RequestUpgrade(context.Background(), 9, domain.LevelIdentity)
	require.NoError(t, err)
	assert.NotEmpty(t, missing)

	_, err = h.svc.RequestUpgrade(context.Background(), 9, domain.LevelBasic)
	assert.ErrorIs(t, err, domain.ErrKYCAlreadyVerified)
}

func TestScreening_PotentialMatchHoldsUpgrade(t *testing.T) {
	h := newHarness()
	p := submitPassport(t, h, 10, "ID1000")
	s := submitSelfie(t, h, 10)
	verifyDoc(t, h, p.ID, "Robert PEP Johnson")
	verifyDoc(t, h, s.ID, "Robert PEP Johnson")

	st, err := h.svc.GetStatus(context.Background(), 10)
	require.NoError(t, err)
	assert.Equal(t, domain.LevelNone, st.Record.Level, "flagged users are never auto-promoted")
	assert.Equal(t, domain.StatusPending, st.Record.Status)
	assert.Equal(t, domain.ScreeningPotentialMatch, st.Record.ScreeningStatus)

	pending, err := h.svc.ListPendingScreenings(context.Background(), 10)
	require.NoError(t, err)
	require.Len(t, pending, 1)

	// Compliance clears → promotion applies.
	_, err = h.svc.ResolveScreening(context.Background(), "compliance-1", pending[0].ID, true, "verified clean")
	require.NoError(t, err)
	st, err = h.svc.GetStatus(context.Background(), 10)
	require.NoError(t, err)
	assert.Equal(t, domain.LevelIdentity, st.Record.Level)
	assert.Equal(t, domain.StatusVerified, st.Record.Status)
}

func TestScreening_TrueMatchBlocks(t *testing.T) {
	h := newHarness()
	p := submitPassport(t, h, 11, "ID1100")
	s := submitSelfie(t, h, 11)
	verifyDoc(t, h, p.ID, "sanctioned person")
	verifyDoc(t, h, s.ID, "sanctioned person")

	st, err := h.svc.GetStatus(context.Background(), 11)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusRejected, st.Record.Status)
	assert.Equal(t, domain.LevelNone, st.Record.Level)

	_, err = h.svc.RequestUpgrade(context.Background(), 11, domain.LevelBasic)
	assert.ErrorIs(t, err, domain.ErrSanctionsMatch)
}

func TestDeleteDocument_Rules(t *testing.T) {
	h := newHarness()
	p := submitPassport(t, h, 12, "ID1200")
	require.NoError(t, h.svc.DeleteDocument(context.Background(), 12, p.ID))

	s := submitSelfie(t, h, 12)
	verifyDoc(t, h, s.ID, "Jane Doe")
	assert.ErrorIs(t, h.svc.DeleteDocument(context.Background(), 12, s.ID), domain.ErrDocumentNotDeletable)

	assert.ErrorIs(t, h.svc.DeleteDocument(context.Background(), 999, p.ID), domain.ErrDocumentNotFound)
	assert.ErrorIs(t, h.svc.DeleteDocument(context.Background(), 12, uuid.New()), domain.ErrDocumentNotFound)
}

func TestExpiry_LazyDowngrade(t *testing.T) {
	h := newHarness()
	p := submitPassport(t, h, 13, "ID1300")
	s := submitSelfie(t, h, 13)
	verifyDoc(t, h, p.ID, "Jane Doe")
	verifyDoc(t, h, s.ID, "Jane Doe")

	// Simulate passport expiry after verification.
	past := time.Now().Add(-time.Hour)
	h.repo.SetDocExpiry(p.ID, &past)

	st, err := h.svc.GetStatus(context.Background(), 13)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusExpired, st.Record.Status)
	assert.Equal(t, domain.LevelNone, st.Record.Level)
}

func TestSweepExpired_MarksBatch(t *testing.T) {
	h := newHarness()
	p := submitPassport(t, h, 14, "ID1400")
	s := submitSelfie(t, h, 14)
	verifyDoc(t, h, p.ID, "Jane Doe")
	verifyDoc(t, h, s.ID, "Jane Doe")

	past := time.Now().Add(-time.Hour)
	h.repo.SetDocExpiry(p.ID, &past)

	n, err := h.svc.SweepExpired(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)
}

func TestProcessWebhook_SignatureAndIdempotency(t *testing.T) {
	h := newHarness()
	ctx := context.Background()
	body := []byte(`{"external_user_id":20,"review_answer":"GREEN","level":3}`)

	// Bad signature rejected.
	err := h.svc.ProcessWebhook(ctx, WebhookInput{
		Provider: "sumsub", ExternalUserID: 20, ReviewAnswer: "GREEN",
		Level: 3, RawBody: body, Signature: "deadbeef",
	})
	assert.ErrorIs(t, err, domain.ErrInvalidWebhookSig)

	// Good signature applied.
	sig := screening.SignForTest(body, "test-secret")
	require.NoError(t, h.svc.ProcessWebhook(ctx, WebhookInput{
		Provider: "sumsub", EventID: "evt-1", ExternalUserID: 20,
		ReviewAnswer: "GREEN", Level: 3, RawBody: body, Signature: sig,
	}))
	st, err := h.svc.GetStatus(ctx, 20)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusVerified, st.Record.Status)
	assert.Equal(t, domain.LevelEnhanced, st.Record.Level)

	// Duplicate delivery rejected (HTTP layer ACKs it).
	err = h.svc.ProcessWebhook(ctx, WebhookInput{
		Provider: "sumsub", EventID: "evt-1", ExternalUserID: 20,
		ReviewAnswer: "GREEN", Level: 3, RawBody: body, Signature: sig,
	})
	assert.ErrorIs(t, err, domain.ErrDuplicateWebhook)

	// Unknown provider rejected.
	assert.ErrorIs(t, h.svc.ProcessWebhook(ctx, WebhookInput{Provider: "other"}), domain.ErrUnsupportedProvider)
}
