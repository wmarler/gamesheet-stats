package gamesheet

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Tokens are the GameSheet API gateway tokens returned by /api/auth/tokens.
type Tokens struct {
	Access  string `json:"access"`
	Refresh string `json:"refresh"`
	Roles   string `json:"roles,omitempty"`
}

// tokenCache is what we persist on disk between runs so that we do not have to
// sign in to Firebase every time.
type tokenCache struct {
	Email  string `json:"email"`
	Tokens Tokens `json:"tokens"`
	Saved  string `json:"saved"`
}

// Authenticator obtains API gateway tokens for the Teams API.
//
// The web app does the following, which we replicate:
//  1. Sign in with Firebase email/password (identitytoolkit REST API) to get
//     a Firebase ID token.
//  2. GET {api}/api/auth/tokens with "Authorization: Bearer <idToken>" to get
//     the gateway's own access/refresh tokens.
//  3. Use the access token for all API calls; when it expires, POST
//     {api}/api/auth/refresh with "Authorization: Bearer <refreshToken>".
type Authenticator struct {
	cfg       *Config
	http      *http.Client
	cachePath string
}

func NewAuthenticator(cfg *Config, httpClient *http.Client, cacheDir string) *Authenticator {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	a := &Authenticator{cfg: cfg, http: httpClient}
	if cacheDir != "" {
		a.cachePath = filepath.Join(cacheDir, "tokens.json")
	}
	return a
}

// Tokens returns a usable set of tokens, preferring cached ones.
func (a *Authenticator) Tokens() (*Tokens, error) {
	if cached := a.loadCache(); cached != nil {
		if !jwtExpired(cached.Access) {
			return cached, nil
		}
		if t, err := a.Refresh(cached.Refresh); err == nil {
			return t, nil
		}
	}
	return a.Login()
}

// Login performs a full sign-in and token exchange.
func (a *Authenticator) Login() (*Tokens, error) {
	idToken, err := a.firebaseSignIn()
	if err != nil {
		return nil, err
	}
	req, _ := http.NewRequest(http.MethodGet, a.cfg.APIBase+"/api/auth/tokens", nil)
	req.Header.Set("Authorization", "Bearer "+idToken)
	t, err := a.doTokenRequest(req, "token exchange")
	if err != nil {
		return nil, err
	}
	a.saveCache(t)
	return t, nil
}

// Refresh exchanges a refresh token for a new token set.
func (a *Authenticator) Refresh(refresh string) (*Tokens, error) {
	if refresh == "" {
		return nil, errors.New("no refresh token")
	}
	req, _ := http.NewRequest(http.MethodPost, a.cfg.APIBase+"/api/auth/refresh", bytes.NewReader([]byte("{}")))
	req.Header.Set("Authorization", "Bearer "+refresh)
	req.Header.Set("Content-Type", "application/json")
	t, err := a.doTokenRequest(req, "token refresh")
	if err != nil {
		return nil, err
	}
	a.saveCache(t)
	return t, nil
}

func (a *Authenticator) doTokenRequest(req *http.Request, what string) (*Tokens, error) {
	resp, err := a.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", what, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: HTTP %d: %s", what, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var out struct {
		Success bool   `json:"success"`
		Tokens  Tokens `json:"tokens"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("%s: bad JSON: %w", what, err)
	}
	if !out.Success || out.Tokens.Access == "" {
		return nil, fmt.Errorf("%s: unsuccessful response: %s", what, out.Error)
	}
	return &out.Tokens, nil
}

func (a *Authenticator) firebaseSignIn() (string, error) {
	payload, _ := json.Marshal(map[string]any{
		"email":             a.cfg.Email,
		"password":          a.cfg.Password,
		"returnSecureToken": true,
	})
	url := "https://identitytoolkit.googleapis.com/v1/accounts:signInWithPassword?key=" + a.cfg.FirebaseAPIKey
	resp, err := a.http.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("firebase sign-in: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		var e struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(body, &e)
		return "", fmt.Errorf("firebase sign-in failed: HTTP %d %s", resp.StatusCode, e.Error.Message)
	}
	var out struct {
		IDToken string `json:"idToken"`
	}
	if err := json.Unmarshal(body, &out); err != nil || out.IDToken == "" {
		return "", errors.New("firebase sign-in: no idToken in response")
	}
	return out.IDToken, nil
}

func (a *Authenticator) loadCache() *Tokens {
	if a.cachePath == "" {
		return nil
	}
	b, err := os.ReadFile(a.cachePath)
	if err != nil {
		return nil
	}
	var c tokenCache
	if json.Unmarshal(b, &c) != nil || c.Email != a.cfg.Email || c.Tokens.Access == "" {
		return nil
	}
	return &c.Tokens
}

func (a *Authenticator) saveCache(t *Tokens) {
	if a.cachePath == "" || t == nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(a.cachePath), 0o700)
	b, _ := json.MarshalIndent(tokenCache{Email: a.cfg.Email, Tokens: *t, Saved: time.Now().Format(time.RFC3339)}, "", "  ")
	_ = os.WriteFile(a.cachePath, b, 0o600)
}

// jwtExpired reports whether a JWT's "exp" claim is in the past (with a small
// safety margin). Tokens that cannot be decoded are treated as expired.
func jwtExpired(token string) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return true
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return true
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if json.Unmarshal(payload, &claims) != nil || claims.Exp == 0 {
		return true
	}
	return time.Now().Add(60*time.Second).Unix() >= claims.Exp
}
