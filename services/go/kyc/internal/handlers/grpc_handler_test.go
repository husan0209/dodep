package handlers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	commonv1 "github.com/opus-casino/proto/gen/go/common/v1"
	pb "github.com/opus-casino/proto/gen/go/kyc/v1"

	"github.com/opus-casino/kyc/internal/domain"
	"github.com/opus-casino/kyc/internal/testutil"
)

func grpcHarness() (*KYCGRPCHandler, *testutil.FakeRepo) {
	repo := testutil.NewFakeRepo()
	return NewKYCGRPCHandler(testService(repo, "s"), nil), repo
}

func uid(v int64) *commonv1.UserId {
	return &commonv1.UserId{Value: itoa(v)}
}

func TestGRPC_SubmitGetListDelete(t *testing.T) {
	h, _ := grpcHarness()
	ctx := context.Background()

	sub, err := h.SubmitDocument(ctx, &pb.SubmitDocumentRequest{
		UserId: uid(5), DocumentType: pb.DocumentType_DOCUMENT_TYPE_PASSPORT,
		DocumentSide: pb.DocumentSide_DOCUMENT_SIDE_FRONT,
		FileId:       "f1", DocumentNumber: "G1", IssuingCountry: "DE",
		ExpiryDate: tsFuture(),
	})
	require.NoError(t, err)
	require.NotNil(t, sub.GetDocument())
	docID := sub.GetDocument().GetId()

	get, err := h.GetDocument(ctx, &pb.GetDocumentRequest{UserId: uid(5), DocumentId: docID})
	require.NoError(t, err)
	assert.Equal(t, docID, get.GetDocument().GetId())
	assert.Empty(t, get.GetDocument().GetDocumentNumber(), "gRPC reads never expose PII")

	// Other users cannot read foreign documents (404, not 403 oracle).
	_, err = h.GetDocument(ctx, &pb.GetDocumentRequest{UserId: uid(6), DocumentId: docID})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())

	list, err := h.ListDocuments(ctx, &pb.ListDocumentsRequest{UserId: uid(5)})
	require.NoError(t, err)
	assert.Len(t, list.GetDocuments(), 1)

	del, err := h.DeleteDocument(ctx, &pb.DeleteDocumentRequest{UserId: uid(5), DocumentId: docID})
	require.NoError(t, err)
	assert.True(t, del.GetSuccess())
}

func TestGRPC_SubmitValidationErrorInBody(t *testing.T) {
	h, _ := grpcHarness()
	// Unspecified type is rejected at the boundary (InvalidArgument status).
	_, err := h.SubmitDocument(context.Background(), &pb.SubmitDocumentRequest{
		UserId: uid(5), DocumentType: pb.DocumentType_DOCUMENT_TYPE_UNSPECIFIED,
	})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())

	// A well-typed but business-invalid submission rides in the Error field.
	sub, err := h.SubmitDocument(context.Background(), &pb.SubmitDocumentRequest{
		UserId: uid(5), DocumentType: pb.DocumentType_DOCUMENT_TYPE_PASSPORT,
		FileId: "f", DocumentNumber: "G9",
		// missing issuing_country + expiry → domain validation error
	})
	require.NoError(t, err, "validation failures ride in the Error field per contract")
	require.NotNil(t, sub.GetError())
}

func TestGRPC_VerifyRejectFlow(t *testing.T) {
	h, repo := grpcHarness()
	ctx := context.Background()

	sub, err := h.SubmitDocument(ctx, &pb.SubmitDocumentRequest{
		UserId: uid(7), DocumentType: pb.DocumentType_DOCUMENT_TYPE_PASSPORT,
		FileId: "f", DocumentNumber: "V7", IssuingCountry: "DE", ExpiryDate: tsFuture(),
	})
	require.NoError(t, err)
	id := sub.GetDocument().GetId()

	ver, err := h.VerifyDocument(ctx, &pb.VerifyDocumentRequest{DocumentId: id, VerifiedBy: "admin"})
	require.NoError(t, err)
	assert.True(t, ver.GetSuccess())

	// Approved documents cannot be deleted (immutable audit trail).
	del, err := h.DeleteDocument(ctx, &pb.DeleteDocumentRequest{UserId: uid(7), DocumentId: id})
	require.NoError(t, err)
	assert.False(t, del.GetSuccess())
	require.NotNil(t, del.GetError())

	// Verify the record advanced via status RPC.
	_ = repo
	stResp, err := h.GetKYCStatus(ctx, &pb.GetKYCStatusRequest{UserId: uid(7)})
	require.NoError(t, err)
	assert.NotNil(t, stResp.GetStatus())
}

func TestGRPC_RejectUnknownDoc(t *testing.T) {
	h, _ := grpcHarness()
	_, err := h.RejectDocument(context.Background(), &pb.RejectDocumentRequest{
		DocumentId: "00000000-0000-0000-0000-000000000000",
		RejectedBy: "admin", RejectionReason: "x",
	})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())
}

func TestGRPC_VerifyMissingReviewer(t *testing.T) {
	h, _ := grpcHarness()
	sub, err := h.SubmitDocument(context.Background(), &pb.SubmitDocumentRequest{
		UserId: uid(9), DocumentType: pb.DocumentType_DOCUMENT_TYPE_SELFIE, FileId: "f",
	})
	require.NoError(t, err)
	_, err = h.VerifyDocument(context.Background(), &pb.VerifyDocumentRequest{
		DocumentId: sub.GetDocument().GetId(),
	})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestGRPC_LevelAndUpgrade(t *testing.T) {
	h, _ := grpcHarness()
	ctx := context.Background()

	lvl, err := h.GetVerificationLevel(ctx, &pb.GetVerificationLevelRequest{UserId: uid(11)})
	require.NoError(t, err)
	assert.Equal(t, pb.KYCLevel_KYC_LEVEL_NONE, lvl.GetLevel())
	require.NotNil(t, lvl.GetLimits())

	up, err := h.UpgradeVerificationLevel(ctx, &pb.UpgradeVerificationLevelRequest{
		UserId: uid(11), TargetLevel: pb.KYCLevel_KYC_LEVEL_BASIC,
	})
	require.NoError(t, err)
	assert.Equal(t, pb.KYCLevel_KYC_LEVEL_BASIC, up.GetNewLevel())

	// Skipping straight to VIP is rejected.
	_, err = h.UpgradeVerificationLevel(ctx, &pb.UpgradeVerificationLevelRequest{
		UserId: uid(11), TargetLevel: pb.KYCLevel_KYC_LEVEL_VIP,
	})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestGRPC_FullIdentityPromotion(t *testing.T) {
	h, _ := grpcHarness()
	ctx := context.Background()

	for _, dt := range []pb.DocumentType{
		pb.DocumentType_DOCUMENT_TYPE_PASSPORT,
		pb.DocumentType_DOCUMENT_TYPE_SELFIE,
	} {
		sub, err := h.SubmitDocument(ctx, &pb.SubmitDocumentRequest{
			UserId: uid(12), DocumentType: dt, FileId: "f",
			DocumentNumber: "ID12", IssuingCountry: "DE", ExpiryDate: tsFuture(),
		})
		require.NoError(t, err)
		_, err = h.VerifyDocument(ctx, &pb.VerifyDocumentRequest{
			DocumentId: sub.GetDocument().GetId(), VerifiedBy: "admin",
		})
		require.NoError(t, err)
	}

	stResp, err := h.GetKYCStatus(ctx, &pb.GetKYCStatusRequest{UserId: uid(12)})
	require.NoError(t, err)
	assert.Equal(t, pb.KYCLevel_KYC_LEVEL_IDENTITY, stResp.GetCurrentLevel())
	assert.Equal(t, pb.KYCVerificationStatus_KYC_VERIFICATION_STATUS_VERIFIED,
		stResp.GetStatus().GetVerificationStatus())
}

func TestGRPC_InvalidUserID(t *testing.T) {
	h, _ := grpcHarness()
	_, err := h.GetKYCStatus(context.Background(), &pb.GetKYCStatusRequest{
		UserId: &commonv1.UserId{Value: "0"},
	})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestGRPC_DocTypeRoundTrip(t *testing.T) {
	for _, dt := range []domain.DocumentType{
		domain.DocPassport, domain.DocDriversLicense, domain.DocNationalID,
		domain.DocResidencePermit, domain.DocProofOfAddress, domain.DocBankStatement,
		domain.DocSelfie, domain.DocSourceOfFunds,
	} {
		assert.Equal(t, dt, protoToDocType(docTypeToProto(dt)), "type %q must round-trip", dt)
	}
	for _, l := range []domain.KYCLevel{
		domain.LevelNone, domain.LevelBasic, domain.LevelIdentity,
		domain.LevelEnhanced, domain.LevelVIP,
	} {
		assert.Equal(t, l, protoToLevel(levelToProto(l)), "level %v must round-trip", l)
	}
}
