package handler

import (
	"errors"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

var (
	errGoogleOAuthDisabled      = infraerrors.NotFound("GOOGLE_OAUTH_DISABLED", "google oauth login is disabled")
	errGoogleOAuthConfigInvalid = infraerrors.InternalServer("GOOGLE_OAUTH_CONFIG_INVALID", "google oauth client id not configured")
	errGoogleIDTokenRequired    = infraerrors.BadRequest("GOOGLE_ID_TOKEN_REQUIRED", "google id token is required")
	errGoogleEmailMissing       = infraerrors.Unauthorized("GOOGLE_EMAIL_MISSING", "google account email is unavailable")
	errGoogleEmailNotVerified   = infraerrors.Unauthorized("GOOGLE_EMAIL_NOT_VERIFIED", "google account email is not verified")
	errGoogleIDTokenInvalid     = infraerrors.Unauthorized("GOOGLE_ID_TOKEN_INVALID", "invalid google id token")
)

type GoogleOAuthExchangeRequest struct {
	GoogleToken    string `json:"google_token" binding:"required"`
	InvitationCode string `json:"invitation_code"`
	Email          string `json:"email"`
	Name           string `json:"name"`
	Avatar         string `json:"avatar"`
}

type GoogleOAuthPendingResponse struct {
	RequiresInvitation bool   `json:"requires_invitation"`
	PendingOAuthToken  string `json:"pending_oauth_token"`
}

type completeGoogleOAuthRegistrationRequest struct {
	PendingOAuthToken string `json:"pending_oauth_token" binding:"required"`
	InvitationCode    string `json:"invitation_code" binding:"required"`
}

// GoogleOAuthExchange verifies a Google ID token and exchanges it for a sub2api token pair.
// POST /api/v1/auth/oauth/google
func (h *AuthHandler) GoogleOAuthExchange(c *gin.Context) {
	var req GoogleOAuthExchangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	clientID, err := h.googleOAuthClientID()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if h.googleTokenVerifier == nil {
		response.ErrorFrom(c, infraerrors.InternalServer("GOOGLE_OAUTH_NOT_READY", "google oauth verifier is not configured"))
		return
	}
	if h.oauthIdentityService == nil {
		response.ErrorFrom(c, infraerrors.InternalServer("GOOGLE_OAUTH_NOT_READY", "oauth identity service is not configured"))
		return
	}

	profile, err := h.googleTokenVerifier.VerifyIDToken(c.Request.Context(), clientID, req.GoogleToken)
	if err != nil {
		response.ErrorFrom(c, errGoogleIDTokenInvalid.WithCause(err))
		return
	}

	email := strings.TrimSpace(profile.Email)
	if email == "" {
		response.ErrorFrom(c, errGoogleEmailMissing)
		return
	}
	if !profile.EmailVerified {
		response.ErrorFrom(c, errGoogleEmailNotVerified)
		return
	}

	username := googleOAuthUsername(profile)
	tokenPair, user, err := h.oauthIdentityService.LoginOrRegisterOAuthWithTokenPair(
		c.Request.Context(),
		email,
		username,
		strings.TrimSpace(req.InvitationCode),
	)
	if err != nil {
		if errors.Is(err, service.ErrOAuthInvitationRequired) {
			pendingToken, tokenErr := h.oauthIdentityService.CreatePendingOAuthToken(email, username)
			if tokenErr != nil {
				response.ErrorFrom(c, infraerrors.InternalServer("GOOGLE_PENDING_OAUTH_TOKEN_FAILED", "failed to create pending oauth token").WithCause(tokenErr))
				return
			}
			response.Success(c, GoogleOAuthPendingResponse{
				RequiresInvitation: true,
				PendingOAuthToken:  pendingToken,
			})
			return
		}
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, authResponseFromTokenPair(tokenPair, user))
}

// CompleteGoogleOAuthRegistration completes an invitation-gated Google OAuth registration.
// POST /api/v1/auth/oauth/google/complete-registration
func (h *AuthHandler) CompleteGoogleOAuthRegistration(c *gin.Context) {
	var req completeGoogleOAuthRegistrationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if h.oauthIdentityService == nil {
		response.ErrorFrom(c, infraerrors.InternalServer("GOOGLE_OAUTH_NOT_READY", "oauth identity service is not configured"))
		return
	}

	email, username, err := h.oauthIdentityService.VerifyPendingOAuthToken(req.PendingOAuthToken)
	if err != nil {
		response.ErrorFrom(c, service.ErrInvalidToken)
		return
	}

	tokenPair, user, err := h.oauthIdentityService.LoginOrRegisterOAuthWithTokenPair(
		c.Request.Context(),
		email,
		username,
		strings.TrimSpace(req.InvitationCode),
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, authResponseFromTokenPair(tokenPair, user))
}

func (h *AuthHandler) googleOAuthClientID() (string, error) {
	if h == nil || h.cfg == nil || !h.cfg.GoogleOAuth.Enabled {
		return "", errGoogleOAuthDisabled
	}
	clientID := strings.TrimSpace(h.cfg.GoogleOAuth.ClientID)
	if clientID == "" {
		return "", errGoogleOAuthConfigInvalid
	}
	return clientID, nil
}

func authResponseFromTokenPair(tokenPair *service.TokenPair, user *service.User) AuthResponse {
	resp := AuthResponse{
		TokenType: "Bearer",
		User:      dto.UserFromService(user),
	}
	if tokenPair == nil {
		return resp
	}
	resp.AccessToken = tokenPair.AccessToken
	resp.RefreshToken = tokenPair.RefreshToken
	resp.ExpiresIn = tokenPair.ExpiresIn
	return resp
}

func googleOAuthUsername(profile *GoogleOAuthProfile) string {
	if profile == nil {
		return ""
	}
	if name := strings.TrimSpace(profile.Name); name != "" {
		return truncateRunes(name, 100)
	}
	email := strings.TrimSpace(profile.Email)
	if idx := strings.Index(email, "@"); idx > 0 {
		return truncateRunes(email[:idx], 100)
	}
	return "google_user"
}

func truncateRunes(value string, max int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= max {
		return string(runes)
	}
	return string(runes[:max])
}
