package handler

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/pkg/googleoauth"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type GoogleOAuthProfile = googleoauth.Profile

type GoogleTokenVerifier interface {
	VerifyIDToken(ctx context.Context, clientID string, rawIDToken string) (*GoogleOAuthProfile, error)
}

type OAuthIdentityService interface {
	LoginOrRegisterOAuthWithTokenPair(ctx context.Context, email, username, invitationCode string) (*service.TokenPair, *service.User, error)
	CreatePendingOAuthToken(email, username string) (string, error)
	VerifyPendingOAuthToken(token string) (email, username string, err error)
}
