//go:build unit

package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type redeemTrialRepoStub struct {
	usageByKey          map[string]*RedeemCodeUsage
	createdUsages       []*RedeemCodeUsage
	updatedCodes        []*RedeemCode
	listByUserCodes     []RedeemCode
	listUsagesByUserOut []RedeemCodeUsage
}

func (s *redeemTrialRepoStub) usageKey(redeemCodeID, userID int64) string {
	return fmt.Sprintf("%d:%d", redeemCodeID, userID)
}

func (s *redeemTrialRepoStub) Create(context.Context, *RedeemCode) error { panic("unexpected Create call") }
func (s *redeemTrialRepoStub) CreateBatch(context.Context, []RedeemCode) error {
	panic("unexpected CreateBatch call")
}
func (s *redeemTrialRepoStub) GetByID(context.Context, int64) (*RedeemCode, error) {
	panic("unexpected GetByID call")
}
func (s *redeemTrialRepoStub) GetByCode(context.Context, string) (*RedeemCode, error) {
	panic("unexpected GetByCode call")
}
func (s *redeemTrialRepoStub) Delete(context.Context, int64) error { panic("unexpected Delete call") }
func (s *redeemTrialRepoStub) Use(context.Context, int64, int64) error {
	panic("unexpected Use call")
}
func (s *redeemTrialRepoStub) List(context.Context, pagination.PaginationParams) ([]RedeemCode, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}
func (s *redeemTrialRepoStub) ListWithFilters(context.Context, pagination.PaginationParams, string, string, string) ([]RedeemCode, *pagination.PaginationResult, error) {
	panic("unexpected ListWithFilters call")
}
func (s *redeemTrialRepoStub) ListByUser(_ context.Context, _ int64, limit int) ([]RedeemCode, error) {
	out := append([]RedeemCode(nil), s.listByUserCodes...)
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
func (s *redeemTrialRepoStub) ListUsagesByUser(_ context.Context, _ int64, limit int) ([]RedeemCodeUsage, error) {
	out := append([]RedeemCodeUsage(nil), s.listUsagesByUserOut...)
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
func (s *redeemTrialRepoStub) ListByUserPaginated(context.Context, int64, pagination.PaginationParams, string) ([]RedeemCode, *pagination.PaginationResult, error) {
	panic("unexpected ListByUserPaginated call")
}
func (s *redeemTrialRepoStub) SumPositiveBalanceByUser(context.Context, int64) (float64, error) {
	panic("unexpected SumPositiveBalanceByUser call")
}
func (s *redeemTrialRepoStub) Update(_ context.Context, code *RedeemCode) error {
	clone := *code
	s.updatedCodes = append(s.updatedCodes, &clone)
	return nil
}
func (s *redeemTrialRepoStub) CreateUsage(_ context.Context, usage *RedeemCodeUsage) error {
	clone := *usage
	clone.ID = int64(len(s.createdUsages) + 1)
	usage.ID = clone.ID
	s.createdUsages = append(s.createdUsages, &clone)
	if s.usageByKey == nil {
		s.usageByKey = map[string]*RedeemCodeUsage{}
	}
	s.usageByKey[s.usageKey(usage.RedeemCodeID, usage.UserID)] = &clone
	return nil
}
func (s *redeemTrialRepoStub) GetUsageByRedeemCodeAndUser(_ context.Context, redeemCodeID, userID int64) (*RedeemCodeUsage, error) {
	if s.usageByKey == nil {
		return nil, nil
	}
	if usage, ok := s.usageByKey[s.usageKey(redeemCodeID, userID)]; ok {
		clone := *usage
		return &clone, nil
	}
	return nil, nil
}

type redeemTrialAPIKeyIssuerStub struct {
	key         *APIKey
	err         error
	createCalls int
	lastUserID  int64
	lastReq     CreateAPIKeyRequest
}

func (s *redeemTrialAPIKeyIssuerStub) Create(_ context.Context, userID int64, req CreateAPIKeyRequest) (*APIKey, error) {
	s.createCalls++
	s.lastUserID = userID
	s.lastReq = req
	if s.err != nil {
		return nil, s.err
	}
	if s.key == nil {
		return nil, nil
	}
	clone := *s.key
	return &clone, nil
}

func TestRedeemAPIKeyTrialTx_Success(t *testing.T) {
	repo := &redeemTrialRepoStub{}
	issuer := &redeemTrialAPIKeyIssuerStub{
		key: &APIKey{ID: 88, Key: "sk-trial-issued", Quota: 20, Status: StatusActive},
	}
	svc := &RedeemService{redeemRepo: repo, apiKeyIssuer: issuer}

	code := &RedeemCode{
		ID:        7,
		Code:      "TRIAL-CODE",
		Type:      RedeemTypeAPIKeyTrial,
		Status:    StatusUnused,
		MaxUses:   100,
		UsedCount: 0,
	}

	issued, err := svc.redeemAPIKeyTrialTx(context.Background(), 1001, code)
	require.NoError(t, err)
	require.NotNil(t, issued)
	require.Equal(t, int64(88), issued.ID)
	require.Equal(t, 1, issuer.createCalls)
	require.Equal(t, int64(1001), issuer.lastUserID)
	require.Equal(t, 20.0, issuer.lastReq.Quota)
	require.NotNil(t, issuer.lastReq.ExpiresInDays)
	require.Equal(t, 7, *issuer.lastReq.ExpiresInDays)
	require.Len(t, repo.createdUsages, 1)
	require.Len(t, repo.updatedCodes, 1)
	require.Equal(t, 1, repo.updatedCodes[0].UsedCount)
	require.Equal(t, StatusUnused, repo.updatedCodes[0].Status)
	require.NotNil(t, code.IssuedAPIKey)
	require.Equal(t, "sk-trial-issued", code.IssuedAPIKey.Key)
}

func TestRedeemAPIKeyTrialTx_RejectsDuplicateUser(t *testing.T) {
	repo := &redeemTrialRepoStub{
		usageByKey: map[string]*RedeemCodeUsage{
			"7:1001": {
				RedeemCodeID: 7,
				UserID:       1001,
				APIKeyID:     9,
			},
		},
	}
	issuer := &redeemTrialAPIKeyIssuerStub{
		key: &APIKey{ID: 88, Key: "sk-trial-issued", Quota: 20, Status: StatusActive},
	}
	svc := &RedeemService{redeemRepo: repo, apiKeyIssuer: issuer}

	_, err := svc.redeemAPIKeyTrialTx(context.Background(), 1001, &RedeemCode{
		ID:        7,
		Type:      RedeemTypeAPIKeyTrial,
		Status:    StatusUnused,
		MaxUses:   100,
		UsedCount: 1,
	})
	require.ErrorIs(t, err, ErrRedeemCodeAlreadyRedeemed)
	require.Equal(t, 0, issuer.createCalls)
	require.Len(t, repo.createdUsages, 0)
	require.Len(t, repo.updatedCodes, 0)
}

func TestRedeemAPIKeyTrialTx_StopsWhenIssuerFails(t *testing.T) {
	repo := &redeemTrialRepoStub{}
	svc := &RedeemService{
		redeemRepo:   repo,
		apiKeyIssuer: &redeemTrialAPIKeyIssuerStub{err: errors.New("issuer down")},
	}

	_, err := svc.redeemAPIKeyTrialTx(context.Background(), 1001, &RedeemCode{
		ID:        7,
		Type:      RedeemTypeAPIKeyTrial,
		Status:    StatusUnused,
		MaxUses:   100,
		UsedCount: 0,
	})
	require.ErrorContains(t, err, "issuer down")
	require.Len(t, repo.createdUsages, 0)
	require.Len(t, repo.updatedCodes, 0)
}

func TestRedeemService_GetUserHistory_MergesTrialUsages(t *testing.T) {
	legacyUsedAt := time.Date(2026, 4, 1, 8, 0, 0, 0, time.UTC)
	trialUsedAt := legacyUsedAt.Add(2 * time.Hour)
	repo := &redeemTrialRepoStub{
		listByUserCodes: []RedeemCode{
			{
				ID:        1,
				Code:      "LEGACY",
				Type:      RedeemTypeBalance,
				Status:    StatusUsed,
				UsedAt:    &legacyUsedAt,
				CreatedAt: legacyUsedAt.Add(-time.Hour),
			},
		},
		listUsagesByUserOut: []RedeemCodeUsage{
			{
				ID:       2,
				UserID:   1001,
				APIKeyID: 55,
				UsedAt:   trialUsedAt,
				RedeemCode: &RedeemCode{
					ID:        2,
					Code:      "TRIAL",
					Type:      RedeemTypeAPIKeyTrial,
					Status:    StatusUnused,
					MaxUses:   100,
					UsedCount: 1,
					CreatedAt: legacyUsedAt,
					IssuedAPIKey: &APIKey{
						ID: 55,
					},
				},
			},
		},
	}
	svc := &RedeemService{redeemRepo: repo}

	history, err := svc.GetUserHistory(context.Background(), 1001, 10)
	require.NoError(t, err)
	require.Len(t, history, 2)
	require.Equal(t, "TRIAL", history[0].Code)
	require.Equal(t, StatusUsed, history[0].Status)
	require.NotNil(t, history[0].UsedAt)
	require.True(t, history[0].UsedAt.Equal(trialUsedAt))
	require.Nil(t, history[0].IssuedAPIKey)
	require.Equal(t, "LEGACY", history[1].Code)
}
