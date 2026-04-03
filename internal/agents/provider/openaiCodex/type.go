package openaicodex

import "time"

const (
	tokenKey    = "agenvoy.codex.token"
	clientID    = "app_EMoamEEZ73f0CkXaXp7hrann"
	authURL     = "https://auth.openai.com/oauth/authorize"
	tokenURL    = "https://auth.openai.com/oauth/token"
	redirectURI = "http://localhost:1455/auth/callback"
	scopes      = "openid email profile offline_access"
)

type StoredToken struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	IDToken      string    `json:"id_token"`
	AccountID    string    `json:"account_id"`
	ExpiresAt    time.Time `json:"expires_at"`
}

func (t *StoredToken) expired() bool {
	return time.Now().After(t.ExpiresAt.Add(-60 * time.Second))
}

type oauthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	Error        string `json:"error,omitempty"`
	ErrorDesc    string `json:"error_description,omitempty"`
}
