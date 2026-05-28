package copilot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"github.com/pardnchiu/go-pkg/filesystem/keychain"
	go_pkg_http "github.com/pardnchiu/go-pkg/http"
)

const (
	deviceCodeAPI       = "https://github.com/login/device/code"
	oauthAccessTokenAPI = "https://github.com/login/oauth/access_token"
	clientID            = "Iv1.b507a08c87ecfe98" // TODO: will replace with personal client id
)

var (
	errAuthorizationPending = fmt.Errorf("authorization pending") // * pre declare error for ensuring padding wont cause login exit
)

type DeviceCode struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

func (c *Agent) Login(ctx context.Context) (*Token, error) {
	return c.LoginWithCallback(ctx, func(code *DeviceCode) {
		expires := time.Now().Add(time.Duration(code.ExpiresIn) * time.Second)
		fmt.Printf("[*] url:      %-36s\n", code.VerificationURI)
		fmt.Printf("[*] code:     %-36s\n", code.UserCode)
		fmt.Printf("[*] expires:  %-36s\n", expires.Format("2006-01-02 15:04:05"))
		fmt.Print("[*] press Enter to open browser")
		go func() {
			var input string
			fmt.Scanln(&input)
			OpenBrowser(code.VerificationURI)
		}()
	})
}

func (c *Agent) LoginWithCallback(ctx context.Context, onCode func(*DeviceCode)) (*Token, error) {
	code, _, err := go_pkg_http.POST[DeviceCode](ctx, nil, deviceCodeAPI,
		map[string]string{},
		map[string]any{
			"client_id": clientID,
		}, "form")
	if err != nil {
		return nil, fmt.Errorf("device-code: %w", err)
	}

	if onCode != nil {
		onCode(&code)
	}

	interval := time.Duration(code.Interval) * time.Second
	deadline := time.Now().Add(time.Duration(code.ExpiresIn) * time.Second)

	var token *Token
	client := &http.Client{Timeout: 30 * time.Second}
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}

		token, err = c.getAccessToken(ctx, client, code.DeviceCode)
		if err != nil {
			// * waiting for authorize
			if errors.Is(err, errAuthorizationPending) {
				continue
			}
			return nil, err
		}
		return token, nil
	}
	return nil, fmt.Errorf("device code expired")
}

func OpenBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		fmt.Printf("[!] can not open browser, please open: %-48s\n", url)
		return
	}
	if cmd != nil {
		if err := cmd.Start(); err != nil {
			slog.Warn("openBrowser cmd.Start",
				slog.String("url", url),
				slog.String("error", err.Error()))
		}
	}
}

type GopilotAccessToken struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error"`
}

func (c *Agent) getAccessToken(ctx context.Context, client *http.Client, deviceCode string) (*Token, error) {
	accessToken, _, err := go_pkg_http.POST[GopilotAccessToken](ctx, client, oauthAccessTokenAPI,
		map[string]string{},
		map[string]any{
			"client_id":   clientID,
			"device_code": deviceCode,
			"grant_type":  "urn:ietf:params:oauth:grant-type:device_code",
		}, "form")
	if err != nil {
		return nil, err
	}

	switch accessToken.Error {
	case "":
		token := &Token{
			AccessToken: accessToken.AccessToken,
			TokenType:   accessToken.TokenType,
			Scope:       accessToken.Scope,
		}

		data, err := json.Marshal(token)
		if err != nil {
			return nil, fmt.Errorf("json.Marshal: %w", err)
		}
		if err := keychain.Set(tokenKey, string(data)); err != nil {
			return nil, fmt.Errorf("keychain.Set: %w", err)
		}
		return token, nil

	case "authorization_pending":
		return nil, errAuthorizationPending

	default:
		return nil, fmt.Errorf("accessToken.Error: %s", accessToken.Error)
	}
}
