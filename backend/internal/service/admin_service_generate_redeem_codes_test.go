//go:build unit

package service

import (
	"context"
	"testing"

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

func TestAdminService_GenerateRedeemCodes_APIKeyTrialNormalized(t *testing.T) {
	repo := &redeemRepoStubForGenerate{}
	svc := &adminServiceImpl{redeemCodeRepo: repo}

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
		require.Equal(t, 100, codes[i].MaxUses)
		require.Equal(t, 0, codes[i].UsedCount)
	}
	for i := range repo.created {
		require.Equal(t, RedeemTypeAPIKeyTrial, repo.created[i].Type)
		require.Equal(t, float64(0), repo.created[i].Value)
		require.Equal(t, 100, repo.created[i].MaxUses)
		require.Equal(t, 0, repo.created[i].UsedCount)
	}
}
