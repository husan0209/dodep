package service

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/opus-casino/user/internal/domain"
	"github.com/opus-casino/user/internal/repository"
)

type UserService struct {
	repo *repository.UserRepository
	log  *zap.Logger
}

func NewUserService(repo *repository.UserRepository, log *zap.Logger) *UserService {
	return &UserService{repo: repo, log: log}
}

func (s *UserService) GetUser(ctx context.Context, userID int64) (*domain.User, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}
	return user, nil
}

func (s *UserService) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}
	return user, nil
}

func (s *UserService) UpdateUser(ctx context.Context, req *domain.UpdateUserRequest) (*domain.User, error) {
	user, err := s.repo.UpdateUser(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}

	s.log.Info("User updated", zap.Int64("user_id", req.UserID))
	return user, nil
}

func (s *UserService) DeleteUser(ctx context.Context, userID int64, reason string) error {
	if err := s.repo.SoftDeleteUser(ctx, userID); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}

	s.log.Info("User deleted", zap.Int64("user_id", userID), zap.String("reason", reason))
	return nil
}

func (s *UserService) GetPreferences(ctx context.Context, userID int64) (*domain.UserPreferences, error) {
	return s.repo.GetPreferences(ctx, userID)
}

func (s *UserService) UpdatePreferences(ctx context.Context, pref *domain.UserPreferences) (*domain.UserPreferences, error) {
	if err := s.repo.UpsertPreferences(ctx, pref); err != nil {
		return nil, fmt.Errorf("update preferences: %w", err)
	}
	return s.repo.GetPreferences(ctx, pref.UserID)
}

func (s *UserService) GetLimits(ctx context.Context, userID int64) (*domain.UserLimits, error) {
	return s.repo.GetLimits(ctx, userID)
}

func (s *UserService) SetLimits(ctx context.Context, req *domain.SetLimitsRequest) (*domain.UserLimits, error) {
	if req.SelfExclusion != nil && *req.SelfExclusion {
		s.log.Warn("User self-excluded", zap.Int64("user_id", req.UserID))
	}

	if err := s.repo.SetLimits(ctx, req.UserID, req); err != nil {
		return nil, fmt.Errorf("set limits: %w", err)
	}

	return s.repo.GetLimits(ctx, req.UserID)
}

func (s *UserService) GetActivity(ctx context.Context, userID int64, limit, offset int) ([]map[string]interface{}, int, error) {
	return s.repo.GetActivity(ctx, userID, limit, offset)
}
