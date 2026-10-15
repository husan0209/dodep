package handlers

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonv1 "github.com/opus-casino/proto/gen/go/common/v1"
	pb "github.com/opus-casino/proto/gen/go/kyc/v1"

	"github.com/opus-casino/kyc/internal/domain"
	"github.com/opus-casino/kyc/internal/service"
)

// KYCGRPCHandler implements the kyc.v1.KYCService contract.
// Proto ↔ domain conversion happens here, at the boundary only.
type KYCGRPCHandler struct {
	pb.UnimplementedKYCServiceServer
	svc *service.KYCService
	log *zap.Logger
}

// NewKYCGRPCHandler creates the gRPC handler.
func NewKYCGRPCHandler(svc *service.KYCService, log *zap.Logger) *KYCGRPCHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &KYCGRPCHandler{svc: svc, log: log}
}

func parseUserID(v *commonv1.UserId) (int64, error) {
	if v == nil || v.Value == "" {
		return 0, status.Error(codes.InvalidArgument, "user_id is required")
	}
	id, err := strconv.ParseInt(v.Value, 10, 64)
	if err != nil || id <= 0 {
		return 0, status.Error(codes.InvalidArgument, "user_id must be a positive integer")
	}
	return id, nil
}

func userIDProto(userID int64) *commonv1.UserId {
	return &commonv1.UserId{Value: strconv.FormatInt(userID, 10)}
}

func tsProto(t *time.Time) *timestamppb.Timestamp {
	if t == nil || t.IsZero() {
		return nil
	}
	return timestamppb.New(*t)
}

func errDetail(code commonv1.ErrorCode, msg string) *commonv1.ErrorDetails {
	return &commonv1.ErrorDetails{ErrorCode: code, ErrorMessage: msg}
}

func mapDomainError(err error) error {
	switch {
	case errors.Is(err, domain.ErrDocumentNotFound),
		errors.Is(err, domain.ErrKYCNotFound),
		errors.Is(err, domain.ErrScreeningNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrInvalidDocument),
		errors.Is(err, domain.ErrInvalidLevelTransition),
		errors.Is(err, domain.ErrInvalidUserID):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrDocumentAlreadyUsed),
		errors.Is(err, domain.ErrKYCAlreadyVerified),
		errors.Is(err, domain.ErrDocumentNotDeletable),
		errors.Is(err, domain.ErrScreeningNotPending):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, domain.ErrSanctionsMatch),
		errors.Is(err, domain.ErrKYCRejected):
		return status.Error(codes.PermissionDenied, err.Error())
	default:
		var ve *domain.ValidationError
		if errors.As(err, &ve) {
			return status.Error(codes.InvalidArgument, ve.Error())
		}
		return status.Error(codes.Internal, "internal error")
	}
}

// ─── Domain → proto ───

func levelToProto(l domain.KYCLevel) pb.KYCLevel {
	switch l {
	case domain.LevelNone:
		return pb.KYCLevel_KYC_LEVEL_NONE
	case domain.LevelBasic:
		return pb.KYCLevel_KYC_LEVEL_BASIC
	case domain.LevelIdentity:
		return pb.KYCLevel_KYC_LEVEL_IDENTITY
	case domain.LevelEnhanced:
		return pb.KYCLevel_KYC_LEVEL_ENHANCED
	case domain.LevelVIP:
		return pb.KYCLevel_KYC_LEVEL_VIP
	default:
		return pb.KYCLevel_KYC_LEVEL_UNSPECIFIED
	}
}

func protoToLevel(l pb.KYCLevel) domain.KYCLevel {
	switch l {
	case pb.KYCLevel_KYC_LEVEL_NONE:
		return domain.LevelNone
	case pb.KYCLevel_KYC_LEVEL_BASIC:
		return domain.LevelBasic
	case pb.KYCLevel_KYC_LEVEL_IDENTITY:
		return domain.LevelIdentity
	case pb.KYCLevel_KYC_LEVEL_ENHANCED:
		return domain.LevelEnhanced
	case pb.KYCLevel_KYC_LEVEL_VIP:
		return domain.LevelVIP
	default:
		return domain.KYCLevel(-1)
	}
}

func verificationStatusToProto(s domain.VerificationStatus) pb.KYCVerificationStatus {
	switch s {
	case domain.StatusNotStarted:
		return pb.KYCVerificationStatus_KYC_VERIFICATION_STATUS_NOT_STARTED
	case domain.StatusInProgress:
		return pb.KYCVerificationStatus_KYC_VERIFICATION_STATUS_IN_PROGRESS
	case domain.StatusPending:
		return pb.KYCVerificationStatus_KYC_VERIFICATION_STATUS_PENDING_REVIEW
	case domain.StatusVerified:
		return pb.KYCVerificationStatus_KYC_VERIFICATION_STATUS_VERIFIED
	case domain.StatusRejected:
		return pb.KYCVerificationStatus_KYC_VERIFICATION_STATUS_REJECTED
	case domain.StatusExpired:
		return pb.KYCVerificationStatus_KYC_VERIFICATION_STATUS_EXPIRED
	default:
		return pb.KYCVerificationStatus_KYC_VERIFICATION_STATUS_UNSPECIFIED
	}
}

func docTypeToProto(t domain.DocumentType) pb.DocumentType {
	switch t {
	case domain.DocPassport:
		return pb.DocumentType_DOCUMENT_TYPE_PASSPORT
	case domain.DocDriversLicense:
		return pb.DocumentType_DOCUMENT_TYPE_DRIVERS_LICENSE
	case domain.DocNationalID:
		return pb.DocumentType_DOCUMENT_TYPE_NATIONAL_ID
	case domain.DocResidencePermit:
		return pb.DocumentType_DOCUMENT_TYPE_RESIDENCE_PERMIT
	case domain.DocProofOfAddress:
		return pb.DocumentType_DOCUMENT_TYPE_PROOF_OF_ADDRESS
	case domain.DocBankStatement:
		return pb.DocumentType_DOCUMENT_TYPE_BANK_STATEMENT
	case domain.DocSelfie:
		return pb.DocumentType_DOCUMENT_TYPE_SELFIE
	case domain.DocSourceOfFunds:
		return pb.DocumentType_DOCUMENT_TYPE_SOURCE_OF_FUNDS
	default:
		return pb.DocumentType_DOCUMENT_TYPE_UNSPECIFIED
	}
}

func protoToDocType(t pb.DocumentType) domain.DocumentType {
	switch t {
	case pb.DocumentType_DOCUMENT_TYPE_PASSPORT:
		return domain.DocPassport
	case pb.DocumentType_DOCUMENT_TYPE_DRIVERS_LICENSE:
		return domain.DocDriversLicense
	case pb.DocumentType_DOCUMENT_TYPE_NATIONAL_ID:
		return domain.DocNationalID
	case pb.DocumentType_DOCUMENT_TYPE_RESIDENCE_PERMIT:
		return domain.DocResidencePermit
	case pb.DocumentType_DOCUMENT_TYPE_PROOF_OF_ADDRESS:
		return domain.DocProofOfAddress
	case pb.DocumentType_DOCUMENT_TYPE_BANK_STATEMENT:
		return domain.DocBankStatement
	case pb.DocumentType_DOCUMENT_TYPE_SELFIE:
		return domain.DocSelfie
	case pb.DocumentType_DOCUMENT_TYPE_SOURCE_OF_FUNDS:
		return domain.DocSourceOfFunds
	default:
		return ""
	}
}

func docSideToProto(s domain.DocumentSide) pb.DocumentSide {
	switch s {
	case domain.SideFront:
		return pb.DocumentSide_DOCUMENT_SIDE_FRONT
	case domain.SideBack:
		return pb.DocumentSide_DOCUMENT_SIDE_BACK
	default:
		return pb.DocumentSide_DOCUMENT_SIDE_SINGLE
	}
}

func protoToDocSide(s pb.DocumentSide) domain.DocumentSide {
	switch s {
	case pb.DocumentSide_DOCUMENT_SIDE_FRONT:
		return domain.SideFront
	case pb.DocumentSide_DOCUMENT_SIDE_BACK:
		return domain.SideBack
	default:
		return domain.SideSingle
	}
}

func docStatusToProto(s domain.DocumentStatus) pb.DocumentStatus {
	switch s {
	case domain.DocDraft:
		return pb.DocumentStatus_DOCUMENT_STATUS_DRAFT
	case domain.DocSubmitted:
		return pb.DocumentStatus_DOCUMENT_STATUS_SUBMITTED
	case domain.DocInReview:
		return pb.DocumentStatus_DOCUMENT_STATUS_IN_REVIEW
	case domain.DocApproved:
		return pb.DocumentStatus_DOCUMENT_STATUS_APPROVED
	case domain.DocRejected:
		return pb.DocumentStatus_DOCUMENT_STATUS_REJECTED
	case domain.DocExpired:
		return pb.DocumentStatus_DOCUMENT_STATUS_EXPIRED
	default:
		return pb.DocumentStatus_DOCUMENT_STATUS_UNSPECIFIED
	}
}

func protoToDocStatus(s pb.DocumentStatus) *domain.DocumentStatus {
	var out domain.DocumentStatus
	switch s {
	case pb.DocumentStatus_DOCUMENT_STATUS_DRAFT:
		out = domain.DocDraft
	case pb.DocumentStatus_DOCUMENT_STATUS_SUBMITTED:
		out = domain.DocSubmitted
	case pb.DocumentStatus_DOCUMENT_STATUS_IN_REVIEW:
		out = domain.DocInReview
	case pb.DocumentStatus_DOCUMENT_STATUS_APPROVED:
		out = domain.DocApproved
	case pb.DocumentStatus_DOCUMENT_STATUS_REJECTED:
		out = domain.DocRejected
	case pb.DocumentStatus_DOCUMENT_STATUS_EXPIRED:
		out = domain.DocExpired
	default:
		return nil
	}
	return &out
}

func docSummaryToProto(d *domain.Document) *pb.DocumentSummary {
	return &pb.DocumentSummary{
		Id: d.ID.String(), UserId: userIDProto(d.UserID),
		DocumentType: docTypeToProto(d.Type), Status: docStatusToProto(d.Status),
		SubmittedAt: timestamppb.New(d.SubmittedAt),
		ReviewedAt:  tsProto(d.ReviewedAt), RejectionReason: d.RejectionReason,
	}
}

func docToProto(d *domain.Document, admin bool) *pb.Document {
	out := &pb.Document{
		Id: d.ID.String(), UserId: userIDProto(d.UserID),
		DocumentType: docTypeToProto(d.Type), DocumentSide: docSideToProto(d.Side),
		Status: docStatusToProto(d.Status), FileId: d.FileID,
		IssuingCountry: d.IssuingCountry,
		ExpiryDate:     tsProto(d.ExpiresAt),
		SubmittedAt:    timestamppb.New(d.SubmittedAt),
		ReviewedAt:     tsProto(d.ReviewedAt), ReviewedBy: d.ReviewedBy,
		RejectionReason: d.RejectionReason, RequiredActions: d.RequiredActions,
		ExtractedData: d.ExtractedData, VerificationNotes: d.Notes,
		CreatedAt: timestamppb.New(d.CreatedAt), UpdatedAt: timestamppb.New(d.UpdatedAt),
	}
	if admin {
		// Signed file URL for compliance review only (5-min TTL in production;
		// stubbed as a reference here — object storage wiring is remaining work).
		out.FileUrl = "s3://platform-kyc-documents/" + d.FileID
		out.DocumentNumber = d.DocumentNumber
	}
	return out
}

func limitsToProto(level domain.KYCLevel) *pb.VerificationLimits {
	lim := domain.LimitsForLevel(level)
	money := func(v string) *commonv1.Money { return &commonv1.Money{Amount: v, Currency: "USD"} }
	return &pb.VerificationLimits{
		Level:             levelToProto(level),
		DailyDepositLimit: money(lim.DailyDeposit), DailyWithdrawalLimit: money(lim.DailyWithdrawal),
		MonthlyDepositLimit: money(lim.MonthlyDeposit), MonthlyWithdrawalLimit: money(lim.MonthlyWithdrawal),
		MaxBetLimit:    money(lim.MaxBet),
		CanTradeCrypto: lim.CanTradeCrypto, CanUseBonus: lim.CanUseBonus,
	}
}

func requirementFor(t domain.DocumentType, submitted map[domain.DocumentType]bool, desc string, req *pb.DocumentRequirements) *pb.DocumentRequirement {
	_, isSub := submitted[t]
	return &pb.DocumentRequirement{
		DocumentType: docTypeToProto(t), Description: desc,
		AcceptedFormats: []string{"jpg", "jpeg", "png", "pdf"},
		Requirements:    req, IsRequired: true, IsSubmitted: isSub,
	}
}

func defaultRequirements() *pb.DocumentRequirements {
	return &pb.DocumentRequirements{
		ColorRequired: true, BothSidesRequired: true, SelfieRequired: true,
		ProofOfAddressRequired: false,
		MaxFileSizeMb:          10, MinResolutionWidth: 800, MinResolutionHeight: 600,
	}
}

// ─── RPCs ───

// GetKYCStatus implements kyc.v1.KYCService/GetKYCStatus.
func (h *KYCGRPCHandler) GetKYCStatus(ctx context.Context, req *pb.GetKYCStatusRequest) (*pb.GetKYCStatusResponse, error) {
	userID, err := parseUserID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	st, err := h.svc.GetStatus(ctx, userID)
	if err != nil {
		h.log.Error("GetKYCStatus failed", zap.Error(err), zap.Int64("user_id", userID))
		return nil, mapDomainError(err)
	}
	rec := st.Record
	submitted := map[domain.DocumentType]bool{}
	summaries := make([]*pb.DocumentSummary, 0, len(st.Documents))
	for _, d := range st.Documents {
		submitted[d.Type] = true
		summaries = append(summaries, docSummaryToProto(d))
	}
	now := time.Now()
	missing := domain.RequiredDocTypes(rec.TargetLevel, st.Documents, now)
	if rec.TargetLevel == domain.LevelNone {
		missing = domain.RequiredDocTypes(domain.LevelIdentity, st.Documents, now)
	}
	reqs := make([]*pb.DocumentRequirement, 0, len(missing))
	for _, t := range missing {
		reqs = append(reqs, requirementFor(t, submitted, "Required for verification", defaultRequirements()))
	}
	return &pb.GetKYCStatusResponse{
		Status: &pb.KYCStatus{
			UserId: userIDProto(userID), Level: levelToProto(rec.Level),
			VerificationStatus: verificationStatusToProto(rec.Status),
			SubmittedAt:        tsProto(rec.SubmittedAt), VerifiedAt: tsProto(rec.VerifiedAt),
			LastReviewedAt: tsProto(rec.LastReviewedAt), ReviewedBy: rec.ReviewedBy,
		},
		CurrentLevel:      levelToProto(rec.Level),
		RequiredDocuments: reqs, SubmittedDocuments: summaries,
		LastUpdated:     timestamppb.New(rec.UpdatedAt),
		RejectionReason: rec.RejectionReason,
	}, nil
}

// SubmitDocument implements kyc.v1.KYCService/SubmitDocument.
func (h *KYCGRPCHandler) SubmitDocument(ctx context.Context, req *pb.SubmitDocumentRequest) (*pb.SubmitDocumentResponse, error) {
	userID, err := parseUserID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	docType := protoToDocType(req.GetDocumentType())
	if docType == "" {
		return nil, status.Error(codes.InvalidArgument, "document_type is required")
	}
	var expires *time.Time
	if req.GetExpiryDate() != nil {
		t := req.GetExpiryDate().AsTime()
		expires = &t
	}
	doc, err := h.svc.SubmitDocument(ctx, service.SubmitInput{
		UserID: userID, Type: docType, Side: protoToDocSide(req.GetDocumentSide()),
		FileID: req.GetFileId(), DocumentNumber: req.GetDocumentNumber(),
		IssuingCountry: req.GetIssuingCountry(), ExpiresAt: expires,
		Metadata: req.GetMetadata(),
	})
	if err != nil {
		if errors.Is(err, domain.ErrDocumentAlreadyUsed) {
			return &pb.SubmitDocumentResponse{
				Error: errDetail(commonv1.ErrorCode_ERROR_CODE_TRANSACTION_DUPLICATE, err.Error()),
			}, nil
		}
		var ve *domain.ValidationError
		if errors.As(err, &ve) {
			return &pb.SubmitDocumentResponse{
				Error: errDetail(commonv1.ErrorCode_ERROR_CODE_VALIDATION_ERROR, ve.Error()),
			}, nil
		}
		h.log.Error("SubmitDocument failed", zap.Error(err), zap.Int64("user_id", userID))
		return nil, mapDomainError(err)
	}
	return &pb.SubmitDocumentResponse{Document: docSummaryToProto(doc)}, nil
}

// GetDocument implements kyc.v1.KYCService/GetDocument (owner-checked).
func (h *KYCGRPCHandler) GetDocument(ctx context.Context, req *pb.GetDocumentRequest) (*pb.GetDocumentResponse, error) {
	userID, err := parseUserID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(req.GetDocumentId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "document_id must be UUID")
	}
	st, err := h.svc.GetStatus(ctx, userID)
	if err != nil {
		return nil, mapDomainError(err)
	}
	for _, d := range st.Documents {
		if d.ID == id {
			return &pb.GetDocumentResponse{Document: docToProto(d, false)}, nil
		}
	}
	return nil, status.Error(codes.NotFound, "document not found")
}

// ListDocuments implements kyc.v1.KYCService/ListDocuments (cursor pagination).
func (h *KYCGRPCHandler) ListDocuments(ctx context.Context, req *pb.ListDocumentsRequest) (*pb.ListDocumentsResponse, error) {
	userID, err := parseUserID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	st, err := h.svc.GetStatus(ctx, userID)
	if err != nil {
		return nil, mapDomainError(err)
	}
	docs := st.Documents
	if req.DocumentType != nil {
		t := protoToDocType(req.GetDocumentType())
		filtered := docs[:0:0]
		for _, d := range docs {
			if d.Type == t {
				filtered = append(filtered, d)
			}
		}
		docs = filtered
	}
	if req.Status != nil {
		want := req.GetStatus()
		filtered := docs[:0:0]
		for _, d := range docs {
			if docStatusToProto(d.Status) == want {
				filtered = append(filtered, d)
			}
		}
		docs = filtered
	}
	pageSize := int(req.GetPagination().GetPageSize())
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	cursor := req.GetPagination().GetCursor()
	start := 0
	if cursor != "" {
		for i, d := range docs {
			if d.ID.String() == cursor {
				start = i + 1
				break
			}
		}
	}
	end := start + pageSize
	hasMore := false
	if end < len(docs) {
		hasMore = true
	} else {
		end = len(docs)
	}
	page := docs[start:end]
	out := make([]*pb.DocumentSummary, 0, len(page))
	for _, d := range page {
		out = append(out, docSummaryToProto(d))
	}
	next := ""
	if hasMore && len(page) > 0 {
		next = page[len(page)-1].ID.String()
	}
	return &pb.ListDocumentsResponse{
		Documents: out,
		Pagination: &commonv1.PageResponse{
			NextCursor: next, HasMore: hasMore,
		},
	}, nil
}

// DeleteDocument implements kyc.v1.KYCService/DeleteDocument.
func (h *KYCGRPCHandler) DeleteDocument(ctx context.Context, req *pb.DeleteDocumentRequest) (*pb.DeleteDocumentResponse, error) {
	userID, err := parseUserID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(req.GetDocumentId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "document_id must be UUID")
	}
	if err := h.svc.DeleteDocument(ctx, userID, id); err != nil {
		if errors.Is(err, domain.ErrDocumentNotFound) {
			return nil, status.Error(codes.NotFound, "document not found")
		}
		if errors.Is(err, domain.ErrDocumentNotDeletable) {
			return &pb.DeleteDocumentResponse{
				Success: false,
				Error:   errDetail(commonv1.ErrorCode_ERROR_CODE_INVALID_ARGUMENT, err.Error()),
			}, nil
		}
		return nil, mapDomainError(err)
	}
	return &pb.DeleteDocumentResponse{Success: true}, nil
}

// VerifyDocument implements kyc.v1.KYCService/VerifyDocument (admin).
func (h *KYCGRPCHandler) VerifyDocument(ctx context.Context, req *pb.VerifyDocumentRequest) (*pb.VerifyDocumentResponse, error) {
	id, err := uuid.Parse(req.GetDocumentId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "document_id must be UUID")
	}
	if req.GetVerifiedBy() == "" {
		return nil, status.Error(codes.InvalidArgument, "verified_by is required")
	}
	if _, err := h.svc.VerifyDocument(ctx, service.VerifyInput{
		DocumentID: id, VerifiedBy: req.GetVerifiedBy(),
		Notes: req.GetVerificationNotes(), ExtractedData: req.GetExtractedData(),
	}); err != nil {
		if errors.Is(err, domain.ErrDocumentNotFound) {
			return nil, status.Error(codes.NotFound, "document not found")
		}
		var ve *domain.ValidationError
		if errors.As(err, &ve) {
			return &pb.VerifyDocumentResponse{
				Success: false,
				Error:   errDetail(commonv1.ErrorCode_ERROR_CODE_VALIDATION_ERROR, ve.Error()),
			}, nil
		}
		return nil, mapDomainError(err)
	}
	return &pb.VerifyDocumentResponse{Success: true}, nil
}

// RejectDocument implements kyc.v1.KYCService/RejectDocument (admin).
func (h *KYCGRPCHandler) RejectDocument(ctx context.Context, req *pb.RejectDocumentRequest) (*pb.RejectDocumentResponse, error) {
	id, err := uuid.Parse(req.GetDocumentId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "document_id must be UUID")
	}
	if req.GetRejectedBy() == "" {
		return nil, status.Error(codes.InvalidArgument, "rejected_by is required")
	}
	if _, err := h.svc.RejectDocument(ctx, service.RejectInput{
		DocumentID: id, RejectedBy: req.GetRejectedBy(),
		Reason: req.GetRejectionReason(), RequiredActions: req.GetRequiredActions(),
	}); err != nil {
		if errors.Is(err, domain.ErrDocumentNotFound) {
			return nil, status.Error(codes.NotFound, "document not found")
		}
		var ve *domain.ValidationError
		if errors.As(err, &ve) {
			return &pb.RejectDocumentResponse{
				Success: false,
				Error:   errDetail(commonv1.ErrorCode_ERROR_CODE_VALIDATION_ERROR, ve.Error()),
			}, nil
		}
		return nil, mapDomainError(err)
	}
	return &pb.RejectDocumentResponse{Success: true}, nil
}

// GetVerificationLevel implements kyc.v1.KYCService/GetVerificationLevel.
func (h *KYCGRPCHandler) GetVerificationLevel(ctx context.Context, req *pb.GetVerificationLevelRequest) (*pb.GetVerificationLevelResponse, error) {
	userID, err := parseUserID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	st, missing, err := h.svc.GetVerificationLevel(ctx, userID)
	if err != nil {
		return nil, mapDomainError(err)
	}
	rec := st.Record
	pending := make([]*pb.LevelRequirement, 0)
	next := rec.Level + 1
	if next >= domain.LevelBasic && next <= domain.LevelVIP && len(missing) > 0 {
		types := make([]pb.DocumentType, 0, len(missing))
		for _, t := range missing {
			types = append(types, docTypeToProto(t))
		}
		pending = append(pending, &pb.LevelRequirement{
			Level:             levelToProto(next),
			Description:       fmt.Sprintf("Requirements for %s verification", next.String()),
			RequiredDocuments: types,
			Limits:            limitsToProto(next),
		})
	}
	return &pb.GetVerificationLevelResponse{
		Level:               levelToProto(rec.Level),
		Limits:              limitsToProto(rec.Level),
		PendingRequirements: pending,
	}, nil
}

// UpgradeVerificationLevel implements kyc.v1.KYCService/UpgradeVerificationLevel.
func (h *KYCGRPCHandler) UpgradeVerificationLevel(ctx context.Context, req *pb.UpgradeVerificationLevelRequest) (*pb.UpgradeVerificationLevelResponse, error) {
	userID, err := parseUserID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	target := protoToLevel(req.GetTargetLevel())
	if !target.IsValid() {
		return nil, status.Error(codes.InvalidArgument, "target_level is invalid")
	}
	st, err := h.svc.GetStatus(ctx, userID)
	if err != nil {
		return nil, mapDomainError(err)
	}
	missing, err := h.svc.RequestUpgrade(ctx, userID, target)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrKYCAlreadyVerified):
			return &pb.UpgradeVerificationLevelResponse{
				NewLevel: levelToProto(st.Record.Level),
				Error:    errDetail(commonv1.ErrorCode_ERROR_CODE_KYC_NOT_VERIFIED, err.Error()),
			}, nil
		case errors.Is(err, domain.ErrSanctionsMatch):
			return &pb.UpgradeVerificationLevelResponse{
				NewLevel: levelToProto(st.Record.Level),
				Error:    errDetail(commonv1.ErrorCode_ERROR_CODE_FRAUD_REVIEW_REQUIRED, err.Error()),
			}, nil
		}
		var ve *domain.ValidationError
		if errors.As(err, &ve) {
			return &pb.UpgradeVerificationLevelResponse{
				NewLevel: levelToProto(st.Record.Level),
				Error:    errDetail(commonv1.ErrorCode_ERROR_CODE_VALIDATION_ERROR, ve.Error()),
			}, nil
		}
		return nil, mapDomainError(err)
	}
	// Re-read: Basic upgrades complete synchronously.
	updated, err := h.svc.GetStatus(ctx, userID)
	if err != nil {
		return nil, mapDomainError(err)
	}
	submitted := map[domain.DocumentType]bool{}
	for _, d := range updated.Documents {
		submitted[d.Type] = true
	}
	reqs := make([]*pb.DocumentRequirement, 0, len(missing))
	for _, t := range missing {
		reqs = append(reqs, requirementFor(t, submitted, "Required for verification", defaultRequirements()))
	}
	return &pb.UpgradeVerificationLevelResponse{
		NewLevel:          levelToProto(updated.Record.Level),
		RequiredDocuments: reqs,
	}, nil
}
