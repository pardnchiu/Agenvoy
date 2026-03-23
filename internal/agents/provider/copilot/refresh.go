package copilot

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/pardnchiu/agenvoy/internal/utils"
)

const (
	copilotTokenAPI = "https://api.github.com/copilot_internal/v2/token"
)

type RefreshToken struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
}

func (c *Agent) checkExpires(ctx context.Context) error {
	if c.Refresh == nil || time.Now().Unix() >= c.Refresh.ExpiresAt-60 {
		return c.refresh(ctx)
	}
	return nil
}

func (c *Agent) refresh(ctx context.Context) error {
	token, code, err := utils.GET[RefreshToken](ctx, nil, copilotTokenAPI, map[string]string{
		"Authorization":  "token " + c.Token.AccessToken,
		"Accept":         "application/json",
		"Editor-Version": "vscode/1.95.0",
	})
	if err != nil {
		return fmt.Errorf("utils.GET: %w", err)
	}
	if code == http.StatusUnauthorized {
		ctx2, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		token, err := c.Login(ctx2)
		if err != nil {
			return fmt.Errorf("c.Login: %w", err)
		}
		c.Token = token
		return c.refresh(ctx)
	}
	if code == http.StatusForbidden || code == http.StatusNotFound {
		return fmt.Errorf("utils.GET: token refresh failed")
	}
	if code != http.StatusOK {
		return fmt.Errorf("utils.GET: %d", code)
	}

	c.Refresh = &token

	return nil
}
