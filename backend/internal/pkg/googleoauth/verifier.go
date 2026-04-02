package googleoauth

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/coreos/go-oidc/v3/oidc"
)

const googleIssuerURL = "https://accounts.google.com"

type Profile struct {
	Subject       string
	Email         string
	Name          string
	Picture       string
	HostedDomain  string
	EmailVerified bool
}

type OIDCVerifier struct {
	providerURL string

	once        sync.Once
	provider    *oidc.Provider
	providerErr error
}

func NewOIDCVerifier() *OIDCVerifier {
	return &OIDCVerifier{
		providerURL: googleIssuerURL,
	}
}

func (v *OIDCVerifier) VerifyIDToken(ctx context.Context, clientID string, rawIDToken string) (*Profile, error) {
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		return nil, fmt.Errorf("google oauth client id is required")
	}
	rawIDToken = strings.TrimSpace(rawIDToken)
	if rawIDToken == "" {
		return nil, fmt.Errorf("google id token is required")
	}

	provider, err := v.getProvider(ctx)
	if err != nil {
		return nil, fmt.Errorf("load google oidc provider: %w", err)
	}

	idToken, err := provider.Verifier(&oidc.Config{ClientID: clientID}).Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("verify google id token: %w", err)
	}

	var claims struct {
		Email         string `json:"email"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
		HostedDomain  string `json:"hd"`
		EmailVerified bool   `json:"email_verified"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("decode google id token claims: %w", err)
	}

	return &Profile{
		Subject:       idToken.Subject,
		Email:         strings.TrimSpace(claims.Email),
		Name:          strings.TrimSpace(claims.Name),
		Picture:       strings.TrimSpace(claims.Picture),
		HostedDomain:  strings.TrimSpace(claims.HostedDomain),
		EmailVerified: claims.EmailVerified,
	}, nil
}

func (v *OIDCVerifier) getProvider(ctx context.Context) (*oidc.Provider, error) {
	v.once.Do(func() {
		v.provider, v.providerErr = oidc.NewProvider(ctx, v.providerURL)
	})
	return v.provider, v.providerErr
}
