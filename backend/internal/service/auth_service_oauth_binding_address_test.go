//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type refreshTokenCacheStub struct{}

func (s *refreshTokenCacheStub) StoreRefreshToken(ctx context.Context, tokenHash string, data *RefreshTokenData, ttl time.Duration) error {
	return nil
}

func (s *refreshTokenCacheStub) GetRefreshToken(ctx context.Context, tokenHash string) (*RefreshTokenData, error) {
	return nil, ErrRefreshTokenNotFound
}

func (s *refreshTokenCacheStub) DeleteRefreshToken(ctx context.Context, tokenHash string) error {
	return nil
}

func (s *refreshTokenCacheStub) DeleteUserRefreshTokens(ctx context.Context, userID int64) error {
	return nil
}

func (s *refreshTokenCacheStub) DeleteTokenFamily(ctx context.Context, familyID string) error {
	return nil
}

func (s *refreshTokenCacheStub) AddToUserTokenSet(ctx context.Context, userID int64, tokenHash string, ttl time.Duration) error {
	return nil
}

func (s *refreshTokenCacheStub) AddToFamilyTokenSet(ctx context.Context, familyID string, tokenHash string, ttl time.Duration) error {
	return nil
}

func (s *refreshTokenCacheStub) GetUserTokenHashes(ctx context.Context, userID int64) ([]string, error) {
	return nil, nil
}

func (s *refreshTokenCacheStub) GetFamilyTokenHashes(ctx context.Context, familyID string) ([]string, error) {
	return nil, nil
}

func (s *refreshTokenCacheStub) IsTokenInFamily(ctx context.Context, familyID string, tokenHash string) (bool, error) {
	return false, nil
}

func TestAuthService_LoginOrRegisterOAuthWithTokenPair_AssignsBindingAddressForNewUser(t *testing.T) {
	repo := &userRepoStub{getByEmailErr: ErrUserNotFound, nextID: 9}
	service := newAuthService(repo, map[string]string{
		SettingKeyRegistrationEnabled: "true",
	}, nil)
	service.cfg.Wallet.BindingMnemonic = testBindingMnemonic
	service.refreshTokenCache = &refreshTokenCacheStub{}

	tokenPair, user, err := service.LoginOrRegisterOAuthWithTokenPair(context.Background(), "oauth@test.com", "oauth-user", "")
	require.NoError(t, err)
	require.NotNil(t, tokenPair)
	require.NotEmpty(t, tokenPair.AccessToken)
	require.NotEmpty(t, tokenPair.RefreshToken)
	require.NotNil(t, user)
	require.Equal(t, int64(9), user.ID)
	require.Regexp(t, "^0x[0-9a-f]{40}$", user.BindingAddress)
	require.Len(t, repo.updated, 1)
	require.Equal(t, user.BindingAddress, repo.updated[0].BindingAddress)
}
