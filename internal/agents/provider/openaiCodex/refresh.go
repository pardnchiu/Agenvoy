package openaicodex

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func (a *Agent) ensureFreshToken(ctx context.Context) error {
	if a.token == nil || a.token.expired() {
		if err := a.refreshToken(ctx); err != nil {
			token, loginErr := a.Login(ctx)
			if loginErr != nil {
				return fmt.Errorf("a.Login: %w", loginErr)
			}
			a.token = token
		}
	}
	return nil
}

func (a *Agent) refreshToken(ctx context.Context) error {
	if a.token == nil || a.token.RefreshToken == "" {
		return fmt.Errorf("no refresh token available")
	}

	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {a.token.RefreshToken},
		"client_id":     {clientID},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL,
		strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("http.NewRequestWithContext: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("httpClient.Do: %w", err)
	}
	defer resp.Body.Close()

	var raw oauthTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return fmt.Errorf("json.Decode: %w", err)
	}
	if raw.Error != "" {
		return fmt.Errorf("refresh error %s: %s", raw.Error, raw.ErrorDesc)
	}

	expiry := time.Now().Add(time.Duration(raw.ExpiresIn) * time.Second)
	if raw.ExpiresIn == 0 {
		expiry = time.Now().Add(3600 * time.Second)
	}

	refreshToken := raw.RefreshToken
	if refreshToken == "" {
		refreshToken = a.token.RefreshToken
	}

	accountID := parseAccountID(raw.IDToken)
	if accountID == "" && a.token != nil {
		accountID = a.token.AccountID
	}

	a.token = &StoredToken{
		AccessToken:  raw.AccessToken,
		RefreshToken: refreshToken,
		IDToken:      raw.IDToken,
		AccountID:    accountID,
		ExpiresAt:    expiry,
	}

	return saveToken(a.token)
}
