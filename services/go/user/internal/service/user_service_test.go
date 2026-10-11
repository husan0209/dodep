package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/opus-casino/user/internal/domain"
)

// fakeUserRepo is an in-memory UserRepository (no DB required).
type fakeUserRepo struct {
	user       *domain.User
	userErr    error
	byEmail    *domain.User
	byEmailErr error
	updated    *domain.User
	updateErr  error
	delErr     error
	prefs      *PreferenceStore
	prefsErr   error
	upsertErr  error
	limits     *domain.UserLimits
	limitsErr  error
	setErr     error
	activity   []map[string]interface{}
	actTotal   int
	actErr     error

	upsertedPrefs *domain.UserPreferences
	setLimitsReq  *domain.SetLimitsRequest
	deletedID     int64
}

func (f *fakeUserRepo) GetUserByID(_ context.Context, _ int64) (*domain.User, error) {
	return f.user, f.userErr
}

func (f *fakeUserRepo) GetUserByEmail(_ context.Context, _ string) (*domain.User, error) {
	return f.byEmail, f.byEmailErr
}

func (f *fakeUserRepo) UpdateUser(_ context.Context, req *domain.UpdateUserRequest) (*domain.User, error) {
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	return f.updated, nil
}

func (f *fakeUserRepo) SoftDeleteUser(_ context.Context, userID int64) error {
	f.deletedID = userID
	return f.delErr
}

func (f *fakeUserRepo) GetPreferences(_ context.Context, _ int64) (*domain.UserPreferences, error) {
	if f.prefsErr != nil {
		return nil, f.prefsErr
	}
	if f.prefs != nil {
		return &f.prefs.pref, nil
	}
	return nil, nil
}

func (f *fakeUserRepo) UpsertPreferences(_ context.Context, pref *domain.UserPreferences) error {
	f.upsertedPrefs = pref
	return f.upsertErr
}

func (f *fakeUserRepo) GetLimits(_ context.Context, _ int64) (*domain.UserLimits, error) {
	return f.limits, f.limitsErr
}

func (f *fakeUserRepo) SetLimits(_ context.Context, _ int64, req *domain.SetLimitsRequest) error {
	f.setLimitsReq = req
	return f.setErr
}

func (f *fakeUserRepo) GetActivity(_ context.Context, _ int64, _, _ int) ([]map[string]interface{}, int, error) {
	return f.activity, f.actTotal, f.actErr
}

// PreferenceStore wraps prefs to keep the fake small.
type PreferenceStore struct {
	pref domain.UserPreferences
}

var _ UserRepository = (*fakeUserRepo)(nil)

func testLogger() *zap.Logger {
	log, _ := zap.NewDevelopment()
	return log
}

func sampleUser() *domain.User {
	return &domain.User{
		ID: 42, UUID: "11111111-1111-1111-1111-111111111111",
		Email: "user@example.com", Username: "player42",
		CountryCode: "UA", CurrencyCode: "USD",
		Status:   domain.UserStatusActive,
		KYCLevel: domain.KYCLevelVerified,
		Language: "en", Timezone: "UTC",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
}

func TestGetUser(t *testing.T) {
	sentinel := errors.New("db is down")
	tests := []struct {
		name    string
		repo    *fakeUserRepo
		wantErr string
		wantID  int64
	}{
		{"found", &fakeUserRepo{user: sampleUser()}, "", 42},
		{"not found", &fakeUserRepo{user: nil}, "user not found", 0},
		{"db error", &fakeUserRepo{userErr: sentinel}, "get user", 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewUserService(tc.repo, testLogger())
			got, err := svc.GetUser(context.Background(), 42)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got.ID != tc.wantID {
					t.Fatalf("expected id %d, got %d", tc.wantID, got.ID)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if got != nil {
				t.Fatal("expected nil user on error")
			}
			if tc.name == "db error" && !errors.Is(err, sentinel) {
				t.Fatalf("expected wrapped sentinel, got %v", err)
			}
		})
	}
}

func TestGetUserByEmail(t *testing.T) {
	svc := NewUserService(&fakeUserRepo{byEmail: sampleUser()}, testLogger())
	got, err := svc.GetUserByEmail(context.Background(), "user@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Email != "user@example.com" {
		t.Fatalf("wrong user: %+v", got)
	}

	svc = NewUserService(&fakeUserRepo{byEmail: nil}, testLogger())
	if _, err := svc.GetUserByEmail(context.Background(), "missing@example.com"); err == nil {
		t.Fatal("expected error for missing user")
	}
}

func TestUpdateAndDeleteUser(t *testing.T) {
	repo := &fakeUserRepo{updated: sampleUser()}
	svc := NewUserService(repo, testLogger())
	username := "renamed"
	got, err := svc.UpdateUser(context.Background(), &domain.UpdateUserRequest{UserID: 42, Username: &username})
	if err != nil || got == nil {
		t.Fatalf("update failed: %v", err)
	}

	repoErr := &fakeUserRepo{updateErr: errors.New("db is down")}
	svc = NewUserService(repoErr, testLogger())
	if _, err := svc.UpdateUser(context.Background(), &domain.UpdateUserRequest{UserID: 42}); err == nil {
		t.Fatal("expected update error")
	}

	delRepo := &fakeUserRepo{}
	svc = NewUserService(delRepo, testLogger())
	if err := svc.DeleteUser(context.Background(), 42, "user request"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if delRepo.deletedID != 42 {
		t.Fatalf("expected soft-delete of 42, got %d", delRepo.deletedID)
	}

	delFail := &fakeUserRepo{delErr: errors.New("db is down")}
	svc = NewUserService(delFail, testLogger())
	if err := svc.DeleteUser(context.Background(), 42, "x"); err == nil {
		t.Fatal("expected delete error")
	}
}

func TestPreferencesRoundTrip(t *testing.T) {
	want := domain.UserPreferences{UserID: 42, Language: "uk", Timezone: "Europe/Kyiv"}
	repo := &fakeUserRepo{prefs: &PreferenceStore{pref: want}}
	svc := NewUserService(repo, testLogger())

	got, err := svc.GetPreferences(context.Background(), 42)
	if err != nil || got.Language != "uk" {
		t.Fatalf("get prefs failed: %+v %v", got, err)
	}

	updated := domain.UserPreferences{UserID: 42, Language: "en", Timezone: "UTC"}
	repo2 := &fakeUserRepo{prefs: &PreferenceStore{pref: updated}}
	svc2 := NewUserService(repo2, testLogger())
	got, err = svc2.UpdatePreferences(context.Background(), &updated)
	if err != nil {
		t.Fatalf("update prefs failed: %v", err)
	}
	if repo2.upsertedPrefs.Language != "en" {
		t.Fatalf("upsert not called with new prefs: %+v", repo2.upsertedPrefs)
	}
	if got.Timezone != "UTC" {
		t.Fatalf("expected re-read prefs, got %+v", got)
	}

	failRepo := &fakeUserRepo{upsertErr: errors.New("db is down")}
	svc3 := NewUserService(failRepo, testLogger())
	if _, err := svc3.UpdatePreferences(context.Background(), &updated); err == nil {
		t.Fatal("expected upsert error")
	}
}

func TestSetLimits(t *testing.T) {
	deposit := "100.00"
	selfExclusion := true
	repo := &fakeUserRepo{limits: &domain.UserLimits{UserID: 42, SelfExclusion: true}}
	svc := NewUserService(repo, testLogger())

	got, err := svc.SetLimits(context.Background(), &domain.SetLimitsRequest{
		UserID: 42, DailyDepositLimit: &deposit, SelfExclusion: &selfExclusion,
	})
	if err != nil {
		t.Fatalf("set limits failed: %v", err)
	}
	if repo.setLimitsReq == nil || repo.setLimitsReq.DailyDepositLimit == nil ||
		*repo.setLimitsReq.DailyDepositLimit != "100.00" {
		t.Fatalf("set limits not forwarded: %+v", repo.setLimitsReq)
	}
	if !got.SelfExclusion {
		t.Fatal("expected re-read limits with self-exclusion")
	}

	failRepo := &fakeUserRepo{setErr: errors.New("db is down")}
	svc = NewUserService(failRepo, testLogger())
	if _, err := svc.SetLimits(context.Background(), &domain.SetLimitsRequest{UserID: 42}); err == nil {
		t.Fatal("expected set limits error")
	}
}

func TestGetLimitsAndActivity(t *testing.T) {
	repo := &fakeUserRepo{
		limits:   &domain.UserLimits{UserID: 42},
		activity: []map[string]interface{}{{"id": 1, "action": "login"}},
		actTotal: 1,
	}
	svc := NewUserService(repo, testLogger())

	lim, err := svc.GetLimits(context.Background(), 42)
	if err != nil || lim.UserID != 42 {
		t.Fatalf("get limits failed: %+v %v", lim, err)
	}

	items, total, err := svc.GetActivity(context.Background(), 42, 20, 0)
	if err != nil || total != 1 || len(items) != 1 {
		t.Fatalf("get activity failed: %+v %d %v", items, total, err)
	}
}

func TestNewUserServiceRequiresRepo(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on nil repo")
		}
	}()
	NewUserService(nil, testLogger())
}
