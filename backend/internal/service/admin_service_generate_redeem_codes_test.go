//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type redeemRepoStubForGenerate struct {
	redeemRepoStub
	created []RedeemCode
}

func (s *redeemRepoStubForGenerate) Create(_ context.Context, code *RedeemCode) error {
	if code == nil {
		return nil
	}
	s.created = append(s.created, *code)
	return nil
}

func TestAdminService_GenerateRedeemCodes_APIKeyTrialUsesDefaultPolicy(t *testing.T) {
	repo := &redeemRepoStubForGenerate{}
	cfg := &config.Config{
		Default: config.DefaultConfig{
			APIKeyTrialQuotaUSD:      20,
			APIKeyTrialMaxUses:       77,
			APIKeyTrialExpiresInDays: 7,
		},
	}
	svc := &adminServiceImpl{redeemCodeRepo: repo, cfg: cfg}

	codes, err := svc.GenerateRedeemCodes(context.Background(), &GenerateRedeemCodesInput{
		Count: 2,
		Type:  RedeemTypeAPIKeyTrial,
		Value: 999,
	})
	require.NoError(t, err)
	require.Len(t, codes, 2)
	require.Len(t, repo.created, 2)

	for i := range codes {
		require.Equal(t, RedeemTypeAPIKeyTrial, codes[i].Type)
		require.Equal(t, float64(0), codes[i].Value)
		require.Equal(t, 77, codes[i].MaxUses)
		require.Equal(t, 0, codes[i].UsedCount)
	}
	for i := range repo.created {
		require.Equal(t, RedeemTypeAPIKeyTrial, repo.created[i].Type)
		require.Equal(t, float64(0), repo.created[i].Value)
		require.Equal(t, 77, repo.created[i].MaxUses)
		require.Equal(t, 0, repo.created[i].UsedCount)
	}
}

func TestAdminService_GenerateRedeemCodes_APIKeyTrialUsesSixCharacterCodes(t *testing.T) {
	repo := &redeemRepoStubForGenerate{}
	cfg := &config.Config{
		Default: config.DefaultConfig{
			APIKeyTrialQuotaUSD:      20,
			APIKeyTrialMaxUses:       100,
			APIKeyTrialExpiresInDays: 7,
		},
	}
	svc := &adminServiceImpl{redeemCodeRepo: repo, cfg: cfg}

	codes, err := svc.GenerateRedeemCodes(context.Background(), &GenerateRedeemCodesInput{
		Count: 5,
		Type:  RedeemTypeAPIKeyTrial,
	})
	require.NoError(t, err)
	require.Len(t, codes, 5)

	for _, code := range codes {
		require.Len(t, code.Code, 6)
		require.Regexp(t, `^[A-Z0-9]{6}$`, code.Code)
	}
}
