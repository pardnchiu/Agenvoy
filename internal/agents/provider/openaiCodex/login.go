package openaicodex

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/filesystem/keychain"
)

func (a *Agent) Login(ctx context.Context) (*StoredToken, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return nil, fmt.Errorf("failed to generate PKCE verifier: %w", err)
	}
	verifier := base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])

	b = make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("failed to generate state: %w", err)
	}
	state := base64.RawURLEncoding.EncodeToString(b)

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	srv, err := startCallbackServer(state, codeCh, errCh)
	if err != nil {
		return nil, fmt.Errorf("startCallbackServer: %w", err)
	}
	defer srv.Shutdown(context.Background())

	url := buildAuthURL(challenge, state, redirectURI)

	fmt.Print("[x] OpenAI Codex OAuth: for dev testing, need ChatGPT Pro/Max")
	fmt.Printf("[x] url browser: %s│\n", url)
	fmt.Print("[*] press Enter to open browser")

	openBrowser(url)

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-errCh:
		return nil, fmt.Errorf("callback: %w", err)
	case code := <-codeCh:
		return a.exchangeCode(ctx, code, verifier, redirectURI)
	}
}

func (a *Agent) exchangeCode(ctx context.Context, code, verifier, redirect string) (*StoredToken, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirect},
		"client_id":     {clientID},
		"code_verifier": {verifier},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL,
		strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("http.NewRequestWithContext: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("httpClient.Do: %w", err)
	}
	defer resp.Body.Close()

	var raw oauthTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("json.Decode: %w", err)
	}
	if raw.Error != "" {
		return nil, fmt.Errorf("token error %s: %s", raw.Error, raw.ErrorDesc)
	}

	expiry := time.Now().Add(time.Duration(raw.ExpiresIn) * time.Second)
	if raw.ExpiresIn == 0 {
		expiry = time.Now().Add(3600 * time.Second)
	}

	token := &StoredToken{
		AccessToken:  raw.AccessToken,
		RefreshToken: raw.RefreshToken,
		IDToken:      raw.IDToken,
		AccountID:    parseAccountID(raw.IDToken),
		ExpiresAt:    expiry,
	}

	if err := saveToken(token); err != nil {
		return nil, fmt.Errorf("saveToken: %w", err)
	}
	return token, nil
}

func HasToken() bool {
	return keychain.Get(tokenKey) != ""
}

func ClearToken() error {
	return keychain.Delete(tokenKey)
}

func saveToken(t *StoredToken) error {
	data, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("json.Marshal: %w", err)
	}
	return keychain.Set(tokenKey, string(data))
}

func buildAuthURL(challenge, state, redirect string) string {
	v := url.Values{
		"response_type":         {"code"},
		"client_id":             {clientID},
		"redirect_uri":          {redirect},
		"scope":                 {scopes},
		"state":                 {state},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}
	return authURL + "?" + v.Encode()
}

func startCallbackServer(expectedState string, codeCh chan<- string, errCh chan<- error) (*http.Server, error) {
	listener, err := net.Listen("tcp", "localhost:1455")
	if err != nil {
		return nil, fmt.Errorf("net.Listen: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/auth/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		if errParam := q.Get("error"); errParam != "" {
			desc := q.Get("error_description")
			fmt.Fprintf(w, "授權失敗%s: %s", errParam, desc)
			errCh <- fmt.Errorf("%s: %s", errParam, desc)
			return
		}

		if q.Get("state") != expectedState {
			fmt.Fprint(w, "授權失敗")
			errCh <- fmt.Errorf("state mismatch")
			return
		}

		code := q.Get("code")
		if code == "" {
			fmt.Fprint(w, "授權失敗: 未收到授權碼")
			errCh <- fmt.Errorf("missing code")
			return
		}

		fmt.Fprint(w, "授權成功")
		codeCh <- code
	})

	srv := &http.Server{
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	go srv.Serve(listener)
	return srv, nil
}

func parseAccountID(idToken string) string {
	parts := strings.SplitN(idToken, ".", 3)
	if len(parts) < 2 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims struct {
		Auth struct {
			ChatGPTAccountID string `json:"chatgpt_account_id"`
		} `json:"https://api.openai.com/auth"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return ""
	}
	return claims.Auth.ChatGPTAccountID
}

func openBrowser(link string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", link)
	case "linux":
		cmd = exec.Command("xdg-open", link)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", link)
	default:
		return
	}
	_ = cmd.Start()
}
