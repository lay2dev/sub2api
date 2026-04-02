//go:build unit

package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type googleTokenVerifierStub struct {
	profile *GoogleOAuthProfile
	err     error
	calls   []googleVerifierCall
}

type googleVerifierCall struct {
	clientID string
	token    string
}

func (s *googleTokenVerifierStub) VerifyIDToken(_ context.Context, clientID string, token string) (*GoogleOAuthProfile, error) {
	s.calls = append(s.calls, googleVerifierCall{
		clientID: clientID,
		token:    token,
	})
	if s.err != nil {
		return nil, s.err
	}
	return s.profile, nil
}

type oauthIdentityServiceStub struct {
	tokenPair          *service.TokenPair
	user               *service.User
	loginErr           error
	pendingToken       string
	pendingTokenErr    error
	verifyEmail        string
	verifyUsername     string
	verifyPendingErr   error
	loginCalls         []oauthIdentityLoginCall
	createPendingCalls []oauthIdentityPendingCall
	verifyPendingCalls []string
}

type oauthIdentityLoginCall struct {
	email          string
	username       string
	invitationCode string
}

type oauthIdentityPendingCall struct {
	email    string
	username string
}

func (s *oauthIdentityServiceStub) LoginOrRegisterOAuthWithTokenPair(_ context.Context, email, username, invitationCode string) (*service.TokenPair, *service.User, error) {
	s.loginCalls = append(s.loginCalls, oauthIdentityLoginCall{
		email:          email,
		username:       username,
		invitationCode: invitationCode,
	})
	if s.loginErr != nil {
		return nil, nil, s.loginErr
	}
	return s.tokenPair, s.user, nil
}

func (s *oauthIdentityServiceStub) CreatePendingOAuthToken(email, username string) (string, error) {
	s.createPendingCalls = append(s.createPendingCalls, oauthIdentityPendingCall{
		email:    email,
		username: username,
	})
	if s.pendingTokenErr != nil {
		return "", s.pendingTokenErr
	}
	return s.pendingToken, nil
}

func (s *oauthIdentityServiceStub) VerifyPendingOAuthToken(token string) (string, string, error) {
	s.verifyPendingCalls = append(s.verifyPendingCalls, token)
	if s.verifyPendingErr != nil {
		return "", "", s.verifyPendingErr
	}
	return s.verifyEmail, s.verifyUsername, nil
}

type googleOAuthTestEnvelope[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

func TestGoogleOAuthExchangeSuccess(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	oauthSvc := &oauthIdentityServiceStub{
		tokenPair: &service.TokenPair{
			AccessToken:  "access-token",
			RefreshToken: "refresh-token",
			ExpiresIn:    3600,
		},
		user: &service.User{
			ID:       101,
			Email:    "alice@example.com",
			Username: "Alice Example",
			Role:     service.RoleUser,
			Status:   service.StatusActive,
		},
	}
	verifier := &googleTokenVerifierStub{
		profile: &GoogleOAuthProfile{
			Subject:       "google-subject-1",
			Email:         "alice@example.com",
			Name:          "Alice Example",
			EmailVerified: true,
			Picture:       "https://example.com/avatar.png",
		},
	}

	h := NewAuthHandler(
		&config.Config{
			GoogleOAuth: config.GoogleOAuthConfig{
				Enabled:  true,
				ClientID: "google-client-id",
			},
		},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	h.oauthIdentityService = oauthSvc
	h.googleTokenVerifier = verifier

	router := gin.New()
	router.POST("/api/v1/auth/oauth/google", h.GoogleOAuthExchange)

	body := bytes.NewBufferString(`{"google_token":"id-token-1"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/oauth/google", body)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var payload googleOAuthTestEnvelope[AuthResponse]
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Equal(t, 0, payload.Code)
	require.Equal(t, "access-token", payload.Data.AccessToken)
	require.Equal(t, "refresh-token", payload.Data.RefreshToken)
	require.Equal(t, 3600, payload.Data.ExpiresIn)
	require.NotNil(t, payload.Data.User)
	require.Equal(t, int64(101), payload.Data.User.ID)
	require.Equal(t, "alice@example.com", payload.Data.User.Email)

	require.Len(t, verifier.calls, 1)
	require.Equal(t, "google-client-id", verifier.calls[0].clientID)
	require.Equal(t, "id-token-1", verifier.calls[0].token)
	require.Len(t, oauthSvc.loginCalls, 1)
	require.Equal(t, "alice@example.com", oauthSvc.loginCalls[0].email)
	require.Equal(t, "Alice Example", oauthSvc.loginCalls[0].username)
	require.Empty(t, oauthSvc.loginCalls[0].invitationCode)
}

func TestGoogleOAuthExchangeInvitationRequired(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	oauthSvc := &oauthIdentityServiceStub{
		loginErr:     service.ErrOAuthInvitationRequired,
		pendingToken: "pending-google-token",
	}
	verifier := &googleTokenVerifierStub{
		profile: &GoogleOAuthProfile{
			Subject:       "google-subject-2",
			Email:         "new-user@example.com",
			Name:          "New User",
			EmailVerified: true,
		},
	}

	h := NewAuthHandler(
		&config.Config{
			GoogleOAuth: config.GoogleOAuthConfig{
				Enabled:  true,
				ClientID: "google-client-id",
			},
		},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	h.oauthIdentityService = oauthSvc
	h.googleTokenVerifier = verifier

	router := gin.New()
	router.POST("/api/v1/auth/oauth/google", h.GoogleOAuthExchange)

	body := bytes.NewBufferString(`{"google_token":"id-token-2"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/oauth/google", body)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var payload googleOAuthTestEnvelope[GoogleOAuthPendingResponse]
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Equal(t, 0, payload.Code)
	require.True(t, payload.Data.RequiresInvitation)
	require.Equal(t, "pending-google-token", payload.Data.PendingOAuthToken)

	require.Len(t, oauthSvc.loginCalls, 1)
	require.Len(t, oauthSvc.createPendingCalls, 1)
	require.Equal(t, "new-user@example.com", oauthSvc.createPendingCalls[0].email)
	require.Equal(t, "New User", oauthSvc.createPendingCalls[0].username)
}

func TestGoogleOAuthExchangeRejectsUnverifiedEmail(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	verifier := &googleTokenVerifierStub{
		profile: &GoogleOAuthProfile{
			Subject:       "google-subject-3",
			Email:         "no-verify@example.com",
			Name:          "No Verify",
			EmailVerified: false,
		},
	}

	h := NewAuthHandler(
		&config.Config{
			GoogleOAuth: config.GoogleOAuthConfig{
				Enabled:  true,
				ClientID: "google-client-id",
			},
		},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	h.oauthIdentityService = &oauthIdentityServiceStub{}
	h.googleTokenVerifier = verifier

	router := gin.New()
	router.POST("/api/v1/auth/oauth/google", h.GoogleOAuthExchange)

	body := bytes.NewBufferString(`{"google_token":"id-token-3"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/oauth/google", body)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), "GOOGLE_EMAIL_NOT_VERIFIED")
}

func TestGoogleOAuthCompleteRegistration(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	oauthSvc := &oauthIdentityServiceStub{
		verifyEmail:    "finish@example.com",
		verifyUsername: "Finish User",
		tokenPair: &service.TokenPair{
			AccessToken:  "access-token-2",
			RefreshToken: "refresh-token-2",
			ExpiresIn:    7200,
		},
		user: &service.User{
			ID:       202,
			Email:    "finish@example.com",
			Username: "Finish User",
			Role:     service.RoleUser,
			Status:   service.StatusActive,
		},
	}

	h := NewAuthHandler(
		&config.Config{
			GoogleOAuth: config.GoogleOAuthConfig{
				Enabled:  true,
				ClientID: "google-client-id",
			},
		},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	h.oauthIdentityService = oauthSvc

	router := gin.New()
	router.POST("/api/v1/auth/oauth/google/complete-registration", h.CompleteGoogleOAuthRegistration)

	body := bytes.NewBufferString(`{"pending_oauth_token":"pending-token","invitation_code":"invite-123"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/oauth/google/complete-registration", body)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var payload googleOAuthTestEnvelope[AuthResponse]
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Equal(t, 0, payload.Code)
	require.Equal(t, "access-token-2", payload.Data.AccessToken)
	require.NotNil(t, payload.Data.User)
	require.Equal(t, int64(202), payload.Data.User.ID)
	require.Len(t, oauthSvc.verifyPendingCalls, 1)
	require.Equal(t, "pending-token", oauthSvc.verifyPendingCalls[0])
	require.Len(t, oauthSvc.loginCalls, 1)
	require.Equal(t, "finish@example.com", oauthSvc.loginCalls[0].email)
	require.Equal(t, "Finish User", oauthSvc.loginCalls[0].username)
	require.Equal(t, "invite-123", oauthSvc.loginCalls[0].invitationCode)
}

func TestGoogleOAuthCompleteRegistrationRejectsInvalidPendingToken(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	oauthSvc := &oauthIdentityServiceStub{
		verifyPendingErr: errors.New("invalid token"),
	}

	h := NewAuthHandler(
		&config.Config{
			GoogleOAuth: config.GoogleOAuthConfig{
				Enabled:  true,
				ClientID: "google-client-id",
			},
		},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	h.oauthIdentityService = oauthSvc

	router := gin.New()
	router.POST("/api/v1/auth/oauth/google/complete-registration", h.CompleteGoogleOAuthRegistration)

	body := bytes.NewBufferString(`{"pending_oauth_token":"bad-token","invitation_code":"invite-123"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/oauth/google/complete-registration", body)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), "INVALID_TOKEN")
}
