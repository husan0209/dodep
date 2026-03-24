package handlers

import (
	"context"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/opus-casino/proto/gen/go/auth/v1"
	commonv1 "github.com/opus-casino/proto/gen/go/common/v1"

	"github.com/opus-casino/auth/internal/domain"
	"github.com/opus-casino/auth/internal/service"
)

// AuthGRPCHandler handles gRPC requests for Auth Service
type AuthGRPCHandler struct {
	pb.UnimplementedAuthServiceServer
	service *service.AuthService
	log     *zap.Logger
}

// NewAuthGRPCHandler creates a new gRPC handler
func NewAuthGRPCHandler(svc *service.AuthService, log *zap.Logger) *AuthGRPCHandler {
	return &AuthGRPCHandler{
		service: svc,
		log:     log,
	}
}

// Register creates a new user account
func (h *AuthGRPCHandler) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	result, err := h.service.Register(ctx, &domain.RegisterRequest{
		Email:        req.Email,
		Password:     req.Password,
		Username:     req.Username,
		CountryCode:  req.Country,
		CurrencyCode: req.Currency,
		DeviceID:     req.DeviceId,
		IPAddress:    req.IpAddress,
	})
	if err != nil {
		h.log.Error("Register failed", zap.Error(err))
		return &pb.RegisterResponse{
			Error: &commonv1.ErrorDetails{
				Code:    "AUTH_REGISTER_FAILED",
				Message: err.Error(),
			},
		}, nil
	}

	return &pb.RegisterResponse{
		UserId: &commonv1.UserId{Value: result.UserID},
		Tokens: toProtoToken(result.Tokens),
		Session: toProtoSession(result.Session),
	}, nil
}

// Login authenticates user with credentials
func (h *AuthGRPCHandler) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	var totpCode *string
	if req.TotpCode != nil {
		totpCode = req.TotpCode
	}

	result, err := h.service.Login(ctx, &domain.LoginRequest{
		Email:      req.Email,
		Password:   req.Password,
		DeviceID:   req.DeviceId,
		IPAddress:  req.IpAddress,
		TOTPCode:   totpCode,
		RememberMe: req.RememberMe,
	})
	if err != nil {
		h.log.Error("Login failed", zap.Error(err))
		return &pb.LoginResponse{
			Error: &commonv1.ErrorDetails{
				Code:    "AUTH_LOGIN_FAILED",
				Message: err.Error(),
			},
		}, nil
	}

	resp := &pb.LoginResponse{
		UserId:      &commonv1.UserId{Value: result.UserID},
		Requires2Fa: result.Requires2FA,
	}

	if result.Requires2FA {
		resp.TempToken = result.TempToken
	} else {
		resp.Tokens = toProtoToken(result.Tokens)
		resp.Session = toProtoSession(result.Session)
	}

	return resp, nil
}

// RefreshToken refreshes an access token
func (h *AuthGRPCHandler) RefreshToken(ctx context.Context, req *pb.RefreshTokenRequest) (*pb.RefreshTokenResponse, error) {
	tokens, err := h.service.RefreshTokens(ctx, req.RefreshToken, req.DeviceId)
	if err != nil {
		h.log.Error("RefreshToken failed", zap.Error(err))
		return &pb.RefreshTokenResponse{
			Error: &commonv1.ErrorDetails{
				Code:    "AUTH_REFRESH_FAILED",
				Message: err.Error(),
			},
		}, nil
	}

	return &pb.RefreshTokenResponse{
		Tokens: toProtoToken(tokens),
	}, nil
}

// Logout invalidates a user session
func (h *AuthGRPCHandler) Logout(ctx context.Context, req *pb.LogoutRequest) (*pb.LogoutResponse, error) {
	err := h.service.Logout(ctx, req.UserId.Value, req.SessionId)
	if err != nil {
		h.log.Error("Logout failed", zap.Error(err))
		return &pb.LogoutResponse{
			Success: false,
			Error: &commonv1.ErrorDetails{
				Code:    "AUTH_LOGOUT_FAILED",
				Message: err.Error(),
			},
		}, nil
	}

	return &pb.LogoutResponse{Success: true}, nil
}

// ValidateToken validates an access token
func (h *AuthGRPCHandler) ValidateToken(ctx context.Context, req *pb.ValidateTokenRequest) (*pb.ValidateTokenResponse, error) {
	claims, err := h.service.ValidateToken(ctx, req.AccessToken)
	if err != nil {
		return &pb.ValidateTokenResponse{
			Valid: false,
			Error: &commonv1.ErrorDetails{
				Code:    "AUTH_TOKEN_INVALID",
				Message: err.Error(),
			},
		}, nil
	}

	return &pb.ValidateTokenResponse{
		Valid:     true,
		UserId:    &commonv1.UserId{Value: claims.UserID},
		SessionId: claims.SessionID,
		Claims: map[string]string{
			"device_id": claims.DeviceID,
		},
	}, nil
}

// GetSession returns session information
func (h *AuthGRPCHandler) GetSession(ctx context.Context, req *pb.GetSessionRequest) (*pb.GetSessionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

// Enable2FA enables two-factor authentication
func (h *AuthGRPCHandler) Enable2FA(ctx context.Context, req *pb.Enable2FARequest) (*pb.Enable2FAResponse, error) {
	secret, qrURI, backupCodes, err := h.service.Enable2FA(ctx, req.UserId.Value)
	if err != nil {
		h.log.Error("Enable2FA failed", zap.Error(err))
		return &pb.Enable2FAResponse{
			Error: &commonv1.ErrorDetails{
				Code:    "AUTH_2FA_ENABLE_FAILED",
				Message: err.Error(),
			},
		}, nil
	}

	return &pb.Enable2FAResponse{
		Secret:      secret,
		QrCodeUri:   qrURI,
		BackupCodes: backupCodes,
	}, nil
}

// Verify2FA verifies a TOTP code
func (h *AuthGRPCHandler) Verify2FA(ctx context.Context, req *pb.Verify2FARequest) (*pb.Verify2FAResponse, error) {
	err := h.service.Verify2FA(ctx, req.UserId.Value, req.TotpCode)
	if err != nil {
		h.log.Error("Verify2FA failed", zap.Error(err))
		return &pb.Verify2FAResponse{
			Success: false,
			Error: &commonv1.ErrorDetails{
				Code:    "AUTH_2FA_VERIFY_FAILED",
				Message: err.Error(),
			},
		}, nil
	}

	return &pb.Verify2FAResponse{Success: true}, nil
}

// Disable2FA disables two-factor authentication
func (h *AuthGRPCHandler) Disable2FA(ctx context.Context, req *pb.Disable2FARequest) (*pb.Disable2FAResponse, error) {
	err := h.service.Disable2FA(ctx, req.UserId.Value, req.TotpCode)
	if err != nil {
		h.log.Error("Disable2FA failed", zap.Error(err))
		return &pb.Disable2FAResponse{
			Success: false,
			Error: &commonv1.ErrorDetails{
				Code:    "AUTH_2FA_DISABLE_FAILED",
				Message: err.Error(),
			},
		}, nil
	}

	return &pb.Disable2FAResponse{Success: true}, nil
}

// ChangePassword changes user password
func (h *AuthGRPCHandler) ChangePassword(ctx context.Context, req *pb.ChangePasswordRequest) (*pb.ChangePasswordResponse, error) {
	err := h.service.ChangePassword(ctx, req.UserId.Value, req.CurrentPassword, req.NewPassword)
	if err != nil {
		h.log.Error("ChangePassword failed", zap.Error(err))
		return &pb.ChangePasswordResponse{
			Success: false,
			Error: &commonv1.ErrorDetails{
				Code:    "AUTH_CHANGE_PASSWORD_FAILED",
				Message: err.Error(),
			},
		}, nil
	}

	return &pb.ChangePasswordResponse{Success: true}, nil
}

// ResetPasswordRequest initiates password reset
func (h *AuthGRPCHandler) ResetPasswordRequest(ctx context.Context, req *pb.ResetPasswordRequestRequest) (*pb.ResetPasswordRequestResponse, error) {
	h.service.ResetPasswordRequest(ctx, req.Email, req.IpAddress)
	// Always return success to prevent email enumeration
	return &pb.ResetPasswordRequestResponse{Success: true}, nil
}

// ResetPassword completes password reset
func (h *AuthGRPCHandler) ResetPassword(ctx context.Context, req *pb.ResetPasswordRequest) (*pb.ResetPasswordResponse, error) {
	err := h.service.ResetPassword(ctx, req.Token, req.NewPassword)
	if err != nil {
		h.log.Error("ResetPassword failed", zap.Error(err))
		return &pb.ResetPasswordResponse{
			Success: false,
			Error: &commonv1.ErrorDetails{
				Code:    "AUTH_RESET_PASSWORD_FAILED",
				Message: err.Error(),
			},
		}, nil
	}

	return &pb.ResetPasswordResponse{Success: true}, nil
}

// ============ Helper functions ============

func toProtoToken(tokens *domain.TokenPair) *pb.AuthToken {
	if tokens == nil {
		return nil
	}
	return &pb.AuthToken{
		AccessToken:      tokens.AccessToken,
		RefreshToken:     tokens.RefreshToken,
		ExpiresIn:        tokens.ExpiresIn,
		RefreshExpiresIn: tokens.RefreshExpiresIn,
		TokenType:        tokens.TokenType,
	}
}

func toProtoSession(session *domain.Session) *pb.Session {
	if session == nil {
		return nil
	}
	return &pb.Session{
		SessionId:    session.ID,
		UserId:       &commonv1.UserId{Value: session.UserID},
		DeviceId:     session.DeviceID,
		IpAddress:    session.IPAddress,
		Country:      session.Country,
		CreatedAt:    timestamppb.New(session.CreatedAt),
		ExpiresAt:    timestamppb.New(session.ExpiresAt),
		LastActivity: timestamppb.New(session.LastActivity),
		IsActive:     session.IsActive,
	}
}
