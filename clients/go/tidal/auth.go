package tidal

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	authBaseURL = "https://login.tidal.com/authorize"
	tokenURL    = "https://auth.tidal.com/v1/oauth2/token"
	userInfoURL = "https://openapi.tidal.com/v2/users/me"

	defaultScopes = "collection.read collection.write playlists.read playlists.write search.read user.read"
)

// Credentials holds the authenticated user's Tidal credentials.
type Credentials struct {
	ClientID     string `json:"client_id"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	UserID       string `json:"user_id"`
	CountryCode  string `json:"country_code"`
	ExpiresAt    int64  `json:"expires_at"`
}

// IsExpired reports whether the access token has expired.
func (c *Credentials) IsExpired() bool {
	return time.Now().Unix() >= c.ExpiresAt
}

// PKCEParams holds OAuth2 PKCE parameters.
type PKCEParams struct {
	CodeVerifier  string
	CodeChallenge string
}

// GeneratePKCE generates a PKCE code verifier and challenge.
func GeneratePKCE() (*PKCEParams, error) {
	// Generate 43-128 character random string
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~"
	verifier := make([]byte, 64)
	for i := range verifier {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return nil, fmt.Errorf("generate random: %w", err)
		}
		verifier[i] = charset[n.Int64()]
	}

	// SHA256 hash + base64url encode
	h := sha256.Sum256(verifier)
	challenge := base64.RawURLEncoding.EncodeToString(h[:])

	return &PKCEParams{
		CodeVerifier:  string(verifier),
		CodeChallenge: challenge,
	}, nil
}

// AuthorizeURL builds the OAuth2 authorization URL for PKCE flow.
func AuthorizeURL(clientID string, port int, pkce *PKCEParams) string {
	params := url.Values{
		"client_id":             {clientID},
		"response_type":        {"code"},
		"redirect_uri":         {fmt.Sprintf("http://localhost:%d/callback", port)},
		"scope":                {defaultScopes},
		"code_challenge":       {pkce.CodeChallenge},
		"code_challenge_method": {"S256"},
	}
	return authBaseURL + "?" + params.Encode()
}

// WaitForCallback starts a local HTTP server and blocks until the OAuth callback arrives.
// It returns the authorization code from the callback.
func WaitForCallback(port int) (string, error) {
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "Missing code", http.StatusBadRequest)
			errCh <- fmt.Errorf("callback missing code parameter")
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<html><body><h1>Authenticated!</h1><p>You can close this tab and return to the terminal.</p></body></html>")
		codeCh <- code
	})

	listener, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		return "", fmt.Errorf("failed to listen on port %d: %w", port, err)
	}

	server := &http.Server{Handler: mux}
	go server.Serve(listener)

	select {
	case code := <-codeCh:
		server.Close()
		return code, nil
	case err := <-errCh:
		server.Close()
		return "", err
	}
}

// ExchangeCode exchanges an authorization code for tokens using PKCE.
func ExchangeCode(ctx context.Context, clientID, code, codeVerifier string, port int) (*Credentials, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {fmt.Sprintf("http://localhost:%d/callback", port)},
		"client_id":     {clientID},
		"code_verifier": {codeVerifier},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token exchange request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read token response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("token exchange failed (status %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("parse token response: %w", err)
	}

	creds := &Credentials{
		ClientID:     clientID,
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    time.Now().Unix() + tokenResp.ExpiresIn,
	}

	// Fetch user profile to get user ID and country
	if err := fetchUserProfile(ctx, creds); err != nil {
		// Non-fatal: we have the tokens, just missing profile info
		fmt.Fprintf(os.Stderr, "Warning: could not fetch user profile: %v\n", err)
	}

	return creds, nil
}

// RefreshAccessToken uses the refresh token to get a new access token.
func RefreshAccessToken(ctx context.Context, creds *Credentials) error {
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {creds.RefreshToken},
		"client_id":     {creds.ClientID},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("create refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("refresh request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read refresh response: %w", err)
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("token refresh failed (status %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return fmt.Errorf("parse refresh response: %w", err)
	}

	creds.AccessToken = tokenResp.AccessToken
	if tokenResp.RefreshToken != "" {
		creds.RefreshToken = tokenResp.RefreshToken
	}
	creds.ExpiresAt = time.Now().Unix() + tokenResp.ExpiresIn
	return nil
}

// fetchUserProfile gets the user ID and country code from the /users/me endpoint.
func fetchUserProfile(ctx context.Context, creds *Credentials) error {
	req, err := http.NewRequestWithContext(ctx, "GET", userInfoURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)
	req.Header.Set("Accept", jsonAPIMediaType)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("user profile request failed: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Parse JSON:API response
	var doc struct {
		Data struct {
			ID         string `json:"id"`
			Attributes struct {
				Country string `json:"country"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return err
	}

	creds.UserID = doc.Data.ID
	creds.CountryCode = doc.Data.Attributes.Country
	if creds.CountryCode == "" {
		creds.CountryCode = "US"
	}
	return nil
}

// CredentialsPath returns the default credentials file path.
func CredentialsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "tidal", "credentials.json")
}

// SaveCredentials writes credentials to the default config path.
func SaveCredentials(creds *Credentials) error {
	path := CredentialsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// LoadCredentials reads saved credentials, returns nil if not found.
func LoadCredentials() *Credentials {
	path := CredentialsPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var creds Credentials
	if json.Unmarshal(data, &creds) != nil {
		return nil
	}
	return &creds
}

// EnsureValidToken checks if the token is expired and refreshes it if needed.
// Saves updated credentials to disk after a successful refresh.
func EnsureValidToken(ctx context.Context, creds *Credentials) error {
	if !creds.IsExpired() {
		return nil
	}
	if creds.RefreshToken == "" {
		return fmt.Errorf("access token expired and no refresh token available — run: tidal login")
	}
	if err := RefreshAccessToken(ctx, creds); err != nil {
		return fmt.Errorf("refresh token: %w", err)
	}
	if err := SaveCredentials(creds); err != nil {
		return fmt.Errorf("save refreshed credentials: %w", err)
	}
	return nil
}

// OpenBrowser opens a URL in the user's default browser.
func OpenBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "linux":
		return exec.Command("xdg-open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}
