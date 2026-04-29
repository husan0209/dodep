package service

import (
	"context"
	"fmt"

	"github.com/opus-casino/admin-bff/internal/client"
	commonv1 "github.com/opus-casino/proto/gen/go/common/v1"
	userv1 "github.com/opus-casino/proto/gen/go/user/v1"
)

type UsersService struct {
	userClient   *client.UserClient
	auditService *AuditService
}

func NewUsersService(uc *client.UserClient, as *AuditService) *UsersService {
	return &UsersService{userClient: uc, auditService: as}
}

func (s *UsersService) GetUser(ctx context.Context, userID int64) (*userv1.User, error) {
	return s.userClient.GetUser(ctx, userID)
}

func (s *UsersService) GetUserByEmail(ctx context.Context, email string) (*userv1.User, error) {
	return s.userClient.GetUserByEmail(ctx, email)
}

func (s *UsersService) UpdateUser(ctx context.Context, adminID int64, userID int64, req *userv1.UpdateUserRequest) error {
	if err := s.userClient.UpdateUser(ctx, userID, req); err != nil {
		return err
	}
	s.auditService.Log(ctx, &adminID, "user:update", "user", fmt.Sprintf("%d", userID), nil, "", "")
	return nil
}

func (s *UsersService) DeleteUser(ctx context.Context, adminID int64, userID int64, reason string) error {
	if err := s.userClient.DeleteUser(ctx, userID, reason); err != nil {
		return err
	}
	s.auditService.Log(ctx, &adminID, "user:delete", "user", fmt.Sprintf("%d", userID), map[string]interface{}{"reason": reason}, "", "")
	return nil
}

func (s *UsersService) GetActivity(ctx context.Context, userID int64, pageSize int32, cursor string) ([]*userv1.ActivityEntry, *commonv1.PageResponse, error) {
	return s.userClient.GetActivity(ctx, userID, pageSize, cursor)
}
