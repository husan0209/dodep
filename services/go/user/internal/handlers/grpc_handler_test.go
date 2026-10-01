package handlers

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonv1 "github.com/opus-casino/proto/gen/go/common/v1"
	pb "github.com/opus-casino/proto/gen/go/user/v1"

	"github.com/opus-casino/user/internal/domain"
	"github.com/opus-casino/user/internal/service"
)

// fakeRepo implements service.UserRepository in memory.
type fakeRepo struct {
	users      map[int64]*domain.User
	userErr    error
	updated    *domain.User
	updateErr  error
	updateSeen *domain.UpdateUserRequest
	delErr     error
	prefs      *domain.UserPreferences
	prefsErr   error
	upsertSeen *domain.UserPreferences
	upsertErr  error
	limits     *domain.UserLimits
	limitsErr  error
	setSeen    *domain.SetLimitsRequest
	setErr     error
	activity   []map[string]interface{}
	actTotal   int
	actErr     error
}

func (f *fakeRepo) GetUserByID(_ context.Context, id int64) (*domain.User, error) {
	if f.userErr != nil {
		return nil, f.userErr
	}
	u, ok := f.users[id]
	if !ok {
		return nil, nil
	}
	return u, nil
}

func (f *fakeRepo) GetUserByEmail(_ context.Context, email string) (*domain.User, error) {
	for _, u := range f.users {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, nil
}

func (f *fakeRepo) UpdateUser(_ context.Context, req *domain.UpdateUserRequest) (*domain.User, error) {
	f.updateSeen = req
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	return f.updated, nil
}

func (f *fakeRepo) SoftDeleteUser(_ context.Context, _ int64) error { return f.delErr }

func (f *fakeRepo) GetPreferences(_ context.Context, _ int64) (*domain.UserPreferences, error) {
	return f.prefs, f.prefsErr
}

func (f *fakeRepo) UpsertPreferences(_ context.Context, pref *domain.UserPreferences) error {
	f.upsertSeen = pref
	return f.upsertErr
}

func (f *fakeRepo) GetLimits(_ context.Context, _ int64) (*domain.UserLimits, error) {
	return f.limits, f.limitsErr
}

func (f *fakeRepo) SetLimits(_ context.Context, _ int64, req *domain.SetLimitsRequest) error {
	f.setSeen = req
	return f.setErr
}

func (f *fakeRepo) GetActivity(_ context.Context, _ int64, _, _ int) ([]map[string]interface{}, int, error) {
	return f.activity, f.actTotal, f.actErr
}

var _ service.UserRepository = (*fakeRepo)(nil)

func testHandler(repo *fakeRepo) *UserGRPCHandler {
	log, _ := zap.NewDevelopment()
	return NewUserGRPCHandler(service.NewUserService(repo, log), log)
}

func sampleUser() *domain.User {
	return &domain.User{
		ID: 42, Email: "user@example.com", Username: "player42",
		CountryCode: "UA", CurrencyCode: "USD",
		Status:    domain.UserStatusActive,
		KYCLevel:  domain.KYCLevelVerified,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
}

func uid(s string) *commonv1.UserId { return &commonv1.UserId{Value: s} }

func grpcCode(err error) codes.Code {
	if err == nil {
		return codes.OK
	}
	st, ok := status.FromError(err)
	if !ok {
		return codes.Unknown
	}
	return st.Code()
}

func TestGRPCHandlerGetUser(t *testing.T) {
	h := testHandler(&fakeRepo{users: map[int64]*domain.User{42: sampleUser()}})
	resp, err := h.GetUser(context.Background(), &pb.GetUserRequest{UserId: uid("42")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.User.Email != "user@example.com" || resp.User.Id.Value != "42" {
		t.Fatalf("wrong user: %+v", resp.User)
	}
	if resp.User.Status != pb.UserStatus(pb.UserStatus_value["active"]) {
		t.Fatalf("wrong status mapping: %v", resp.User.Status)
	}

	h = testHandler(&fakeRepo{users: map[int64]*domain.User{}})
	_, err = h.GetUser(context.Background(), &pb.GetUserRequest{UserId: uid("99")})
	if grpcCode(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", err)
	}

	_, err = h.GetUser(context.Background(), &pb.GetUserRequest{UserId: uid("not-a-number")})
	if grpcCode(err) != codes.NotFound {
		t.Fatalf("expected NotFound for bad id, got %v", err)
	}
}

func TestGRPCHandlerGetUserByEmail(t *testing.T) {
	h := testHandler(&fakeRepo{users: map[int64]*domain.User{42: sampleUser()}})
	resp, err := h.GetUserByEmail(context.Background(), &pb.GetUserByEmailRequest{Email: "user@example.com"})
	if err != nil || resp.User == nil || resp.Error != nil {
		t.Fatalf("expected user, got %+v %v", resp, err)
	}

	resp, err = h.GetUserByEmail(context.Background(), &pb.GetUserByEmailRequest{Email: "missing@example.com"})
	if err != nil {
		t.Fatalf("error must be in-body, got gRPC error: %v", err)
	}
	if resp.Error == nil || resp.User != nil {
		t.Fatalf("expected in-body error, got %+v", resp)
	}
}

func TestGRPCHandlerUpdateUser(t *testing.T) {
	repo := &fakeRepo{updated: sampleUser()}
	h := testHandler(repo)
	username := "renamed"
	resp, err := h.UpdateUser(context.Background(), &pb.UpdateUserRequest{UserId: uid("42"), Username: &username})
	if err != nil || resp.User == nil || resp.Error != nil {
		t.Fatalf("expected updated user, got %+v %v", resp, err)
	}
	if repo.updateSeen.Username == nil || *repo.updateSeen.Username != "renamed" {
		t.Fatalf("username not forwarded: %+v", repo.updateSeen)
	}

	h = testHandler(&fakeRepo{updateErr: errors.New("db is down")})
	resp, err = h.UpdateUser(context.Background(), &pb.UpdateUserRequest{UserId: uid("42")})
	if err != nil {
		t.Fatalf("error must be in-body, got gRPC error: %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected in-body error")
	}
}

func TestGRPCHandlerDeleteUser(t *testing.T) {
	h := testHandler(&fakeRepo{})
	resp, err := h.DeleteUser(context.Background(), &pb.DeleteUserRequest{UserId: uid("42"), Reason: "request"})
	if err != nil || !resp.Success {
		t.Fatalf("expected success, got %+v %v", resp, err)
	}

	h = testHandler(&fakeRepo{delErr: errors.New("db is down")})
	resp, err = h.DeleteUser(context.Background(), &pb.DeleteUserRequest{UserId: uid("42")})
	if err != nil {
		t.Fatalf("error must be in-body, got gRPC error: %v", err)
	}
	if resp.Success || resp.Error == nil {
		t.Fatalf("expected failure body, got %+v", resp)
	}
}

func TestGRPCHandlerPreferences(t *testing.T) {
	prefs := &domain.UserPreferences{UserID: 42, Language: "uk", Timezone: "Europe/Kyiv", UpdatedAt: time.Now()}
	h := testHandler(&fakeRepo{prefs: prefs})
	resp, err := h.GetPreferences(context.Background(), &pb.GetPreferencesRequest{UserId: uid("42")})
	if err != nil || resp.Preferences.Language != "uk" {
		t.Fatalf("expected prefs, got %+v %v", resp, err)
	}

	h = testHandler(&fakeRepo{prefsErr: errors.New("db is down")})
	if _, err := h.GetPreferences(context.Background(), &pb.GetPreferencesRequest{UserId: uid("42")}); grpcCode(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v", err)
	}

	repo := &fakeRepo{prefs: prefs}
	h = testHandler(repo)
	marketing := false
	resp2, err := h.UpdatePreferences(context.Background(), &pb.UpdatePreferencesRequest{
		UserId: uid("42"), Language: strPtr("en"), MarketingEmails: &marketing,
	})
	if err != nil || resp2.Preferences == nil || resp2.Error != nil {
		t.Fatalf("expected updated prefs, got %+v %v", resp2, err)
	}
	if repo.upsertSeen.Language != "en" || repo.upsertSeen.MarketingEmails {
		t.Fatalf("prefs not forwarded: %+v", repo.upsertSeen)
	}
}

func TestGRPCHandlerLimits(t *testing.T) {
	limits := &domain.UserLimits{
		UserID: 42, UpdatedAt: time.Now(),
		SessionTimeLimit: &domain.TimeLimit{Minutes: 60, IsActive: true},
	}
	h := testHandler(&fakeRepo{limits: limits})
	resp, err := h.GetLimits(context.Background(), &pb.GetLimitsRequest{UserId: uid("42")})
	if err != nil || resp.Limits.SessionTimeLimit.Minutes != 60 {
		t.Fatalf("expected limits, got %+v %v", resp, err)
	}

	plain := &domain.UserLimits{UserID: 42, UpdatedAt: time.Now()}
	if out := toProtoLimits(plain); out.SessionTimeLimit != nil {
		t.Fatal("expected nil session limit when unset")
	}

	repo := &fakeRepo{limits: limits}
	h = testHandler(repo)
	selfEx := true
	resp2, err := h.SetLimits(context.Background(), &pb.SetLimitsRequest{
		UserId:             uid("42"),
		DailyDepositLimit:  &pb.MoneyLimit{Amount: &commonv1.Money{Amount: "50.00"}},
		SessionTimeLimit:   &pb.TimeLimit{Minutes: 60},
		SelfExclusion:      &selfEx,
		SelfExclusionUntil: timestamppb.New(time.Now().Add(24 * time.Hour)),
	})
	if err != nil || resp2.Limits == nil || resp2.Error != nil {
		t.Fatalf("expected limits, got %+v %v", resp2, err)
	}
	if repo.setSeen.DailyDepositLimit == nil || *repo.setSeen.DailyDepositLimit != "50.00" {
		t.Fatalf("deposit limit not forwarded: %+v", repo.setSeen)
	}
	if repo.setSeen.SessionTimeMinutes == nil || *repo.setSeen.SessionTimeMinutes != 60 {
		t.Fatalf("session minutes not forwarded: %+v", repo.setSeen)
	}

	h = testHandler(&fakeRepo{setErr: errors.New("db is down")})
	resp3, err := h.SetLimits(context.Background(), &pb.SetLimitsRequest{UserId: uid("42")})
	if err != nil || resp3.Error == nil {
		t.Fatalf("expected in-body error, got %+v %v", resp3, err)
	}
}

func TestGRPCHandlerGetActivity(t *testing.T) {
	repo := &fakeRepo{
		activity: []map[string]interface{}{
			{"id": 1, "action": "login"},
			{"id": 2, "action": "deposit"},
		},
		actTotal: 2,
	}
	h := testHandler(repo)
	resp, err := h.GetActivity(context.Background(), &pb.GetActivityRequest{
		UserId: uid("42"), Pagination: &commonv1.PageRequest{PageSize: 20},
	})
	if err != nil || len(resp.Activities) != 2 || resp.Pagination.TotalCount == nil ||
		*resp.Pagination.TotalCount != 2 {
		t.Fatalf("wrong activity mapping: %+v %v", resp, err)
	}
	if resp.Activities[0].Description != "login" || resp.Activities[0].UserId.Value != "42" {
		t.Fatalf("wrong entry mapping: %+v", resp.Activities[0])
	}

	h = testHandler(&fakeRepo{actErr: errors.New("db is down")})
	if _, err := h.GetActivity(context.Background(), &pb.GetActivityRequest{UserId: uid("42")}); grpcCode(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v", err)
	}
}

func strPtr(s string) *string { return &s }
