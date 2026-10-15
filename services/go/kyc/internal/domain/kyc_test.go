package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKYCLevel_CanUpgradeTo(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		from    KYCLevel
		to      KYCLevel
		allowed bool
	}{
		{"none to basic", LevelNone, LevelBasic, true},
		{"basic to identity", LevelBasic, LevelIdentity, true},
		{"identity to enhanced", LevelIdentity, LevelEnhanced, true},
		{"enhanced to vip", LevelEnhanced, LevelVIP, true},
		{"skip level rejected", LevelNone, LevelIdentity, false},
		{"skip two levels rejected", LevelBasic, LevelVIP, false},
		{"downgrade rejected", LevelIdentity, LevelBasic, false},
		{"same level rejected", LevelIdentity, LevelIdentity, false},
		{"beyond vip rejected", LevelVIP, KYCLevel(5), false},
		{"invalid target rejected", LevelNone, KYCLevel(-1), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.allowed, tt.from.CanUpgradeTo(tt.to))
		})
	}
}

func TestDocumentStatus_CanTransitionTo(t *testing.T) {
	t.Parallel()
	tests := []struct {
		from    DocumentStatus
		to      DocumentStatus
		allowed bool
	}{
		{DocDraft, DocSubmitted, true},
		{DocSubmitted, DocInReview, true},
		{DocSubmitted, DocApproved, true},
		{DocSubmitted, DocRejected, true},
		{DocInReview, DocApproved, true},
		{DocInReview, DocRejected, true},
		{DocApproved, DocExpired, true},
		{DocRejected, DocSubmitted, false}, // rejected docs are never rewritten
		{DocApproved, DocRejected, false},
		{DocExpired, DocApproved, false},
		{DocDraft, DocApproved, false},
	}
	for _, tt := range tests {
		name := string(tt.from) + "_to_" + string(tt.to)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.allowed, tt.from.CanTransitionTo(tt.to))
		})
	}
}

func futureDate(days int) *time.Time {
	t := time.Now().AddDate(0, 0, days).Truncate(24 * time.Hour)
	return &t
}

func pastDate(days int) *time.Time {
	t := time.Now().AddDate(0, 0, -days)
	return &t
}

func TestDocument_ValidateSubmit(t *testing.T) {
	t.Parallel()
	now := time.Now()
	tests := []struct {
		name    string
		doc     Document
		wantErr bool
		field   string
	}{
		{
			name: "valid passport",
			doc: Document{UserID: 1, Type: DocPassport, Side: SideFront,
				FileID: "file-123", DocumentNumber: "AB1234567",
				IssuingCountry: "DE", ExpiresAt: futureDate(365)},
		},
		{
			name: "valid selfie without expiry",
			doc:  Document{UserID: 1, Type: DocSelfie, Side: SideSingle, FileID: "file-9"},
		},
		{
			name: "expired passport rejected",
			doc: Document{UserID: 1, Type: DocPassport, Side: SideFront,
				FileID: "f", DocumentNumber: "X1", IssuingCountry: "DE",
				ExpiresAt: pastDate(1)},
			wantErr: true, field: "expiry_date",
		},
		{
			name: "identity doc without number rejected",
			doc: Document{UserID: 1, Type: DocNationalID, Side: SideFront,
				FileID: "f", IssuingCountry: "DE", ExpiresAt: futureDate(30)},
			wantErr: true, field: "document_number",
		},
		{
			name: "identity doc without country rejected",
			doc: Document{UserID: 1, Type: DocDriversLicense, Side: SideFront,
				FileID: "f", DocumentNumber: "D1", ExpiresAt: futureDate(30)},
			wantErr: true, field: "issuing_country",
		},
		{
			name: "lowercase country rejected",
			doc: Document{UserID: 1, Type: DocPassport, Side: SideFront,
				FileID: "f", DocumentNumber: "D1", IssuingCountry: "de",
				ExpiresAt: futureDate(30)},
			wantErr: true, field: "issuing_country",
		},
		{
			name:    "missing file rejected",
			doc:     Document{UserID: 1, Type: DocSelfie, Side: SideSingle},
			wantErr: true, field: "file_id",
		},
		{
			name:    "unknown type rejected",
			doc:     Document{UserID: 1, Type: "library_card", Side: SideSingle, FileID: "f"},
			wantErr: true, field: "document_type",
		},
		{
			name:    "zero user rejected",
			doc:     Document{Type: DocSelfie, Side: SideSingle, FileID: "f"},
			wantErr: true, field: "user_id",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.doc.ValidateSubmit(now)
			if !tt.wantErr {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			var ve *ValidationError
			require.ErrorAs(t, err, &ve)
			found := false
			for _, f := range ve.Fields {
				if f.Field == tt.field {
					found = true
				}
			}
			assert.True(t, found, "expected field %q in %v", tt.field, ve.Fields)
		})
	}
}

func approvedDoc(docType DocumentType, submitted time.Time, expires *time.Time) *Document {
	return &Document{
		Type: docType, Side: SideSingle, Status: DocApproved,
		FileID: "f", SubmittedAt: submitted, ExpiresAt: expires,
	}
}

func TestComputeLevel(t *testing.T) {
	t.Parallel()
	now := time.Now()
	tests := []struct {
		name  string
		docs  []*Document
		level KYCLevel
	}{
		{"empty is none", nil, LevelNone},
		{"id alone is none", []*Document{
			approvedDoc(DocPassport, now, futureDate(365)),
		}, LevelNone},
		{"id plus selfie is identity", []*Document{
			approvedDoc(DocPassport, now, futureDate(365)),
			approvedDoc(DocSelfie, now, nil),
		}, LevelIdentity},
		{"plus address is enhanced", []*Document{
			approvedDoc(DocPassport, now, futureDate(365)),
			approvedDoc(DocSelfie, now, nil),
			approvedDoc(DocProofOfAddress, now, nil),
		}, LevelEnhanced},
		{"plus source of funds is vip", []*Document{
			approvedDoc(DocPassport, now, futureDate(365)),
			approvedDoc(DocSelfie, now, nil),
			approvedDoc(DocBankStatement, now, nil),
			approvedDoc(DocSourceOfFunds, now, nil),
		}, LevelVIP},
		{"expired passport does not count", []*Document{
			approvedDoc(DocPassport, now, pastDate(1)),
			approvedDoc(DocSelfie, now, nil),
		}, LevelNone},
		{"stale address does not count", []*Document{
			approvedDoc(DocPassport, now, futureDate(365)),
			approvedDoc(DocSelfie, now, nil),
			approvedDoc(DocProofOfAddress, now.AddDate(0, 0, -(AddressDocFreshnessDays+1)), nil),
		}, LevelIdentity},
		{"rejected docs do not count", []*Document{
			{Type: DocPassport, Status: DocRejected, SubmittedAt: now},
			approvedDoc(DocSelfie, now, nil),
		}, LevelNone},
		{"bank statement counts as address", []*Document{
			approvedDoc(DocNationalID, now, futureDate(100)),
			approvedDoc(DocSelfie, now, nil),
			approvedDoc(DocBankStatement, now, nil),
		}, LevelEnhanced},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.level, ComputeLevel(tt.docs, now))
		})
	}
}

func TestRequiredDocTypes(t *testing.T) {
	t.Parallel()
	now := time.Now()
	docs := []*Document{
		approvedDoc(DocPassport, now, futureDate(365)),
		approvedDoc(DocSelfie, now, nil),
	}
	assert.Empty(t, RequiredDocTypes(LevelIdentity, docs, now))
	assert.Equal(t, []DocumentType{DocProofOfAddress}, RequiredDocTypes(LevelEnhanced, docs, now))
	assert.Empty(t, RequiredDocTypes(LevelBasic, docs, now))
	assert.Empty(t, RequiredDocTypes(LevelNone, docs, now))
}

func TestLimitsForLevel_NoFloat(t *testing.T) {
	t.Parallel()
	// Limits are decimal strings; a float rendering (e.g. "1e+06") must fail.
	for _, l := range []KYCLevel{LevelNone, LevelBasic, LevelIdentity, LevelEnhanced, LevelVIP} {
		lim := LimitsForLevel(l)
		for _, v := range []string{lim.DailyDeposit, lim.DailyWithdrawal, lim.MonthlyDeposit, lim.MonthlyWithdrawal, lim.MaxBet} {
			assert.NotContains(t, v, "e", "level %v limit %q looks like float notation", l, v)
			assert.NotContains(t, v, ".", "level %v limit %q must be whole units", l, v)
		}
	}
	identity := LimitsForLevel(LevelIdentity)
	assert.Equal(t, "5000", identity.DailyDeposit)
	assert.Equal(t, "2000", identity.DailyWithdrawal)
	none := LimitsForLevel(LevelNone)
	assert.Equal(t, "0", none.DailyDeposit)
	assert.False(t, none.CanUseBonus)
	assert.True(t, LimitsForLevel(LevelEnhanced).CanTradeCrypto)
}

func TestScreeningStatus_BlocksUpgrade(t *testing.T) {
	t.Parallel()
	assert.False(t, ScreeningClear.BlocksUpgrade())
	assert.False(t, ScreeningPending.BlocksUpgrade())
	assert.True(t, ScreeningPotentialMatch.BlocksUpgrade())
	assert.True(t, ScreeningTrueMatch.BlocksUpgrade())
}
