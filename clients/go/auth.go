package qobuz

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	defaultAppID        = "304027809"
	defaultPrivateKey   = "6lz8C03UDIC7"
)

// Credentials holds the authenticated user's Qobuz credentials.
type Credentials struct {
	AppID         string `json:"app_id"`
	UserAuthToken string `json:"user_auth_token"`
	UserID        string `json:"user_id"`
	DisplayName   string `json:"display_name"`
}

// OAuthURL builds the OAuth login URL for the given callback port.
func OAuthURL(port int) string {
	return fmt.Sprintf(
		"https://www.qobuz.com/signin/oauth?ext_app_id=%s&redirect_url=http://localhost:%d/callback",
		defaultAppID, port,
	)
}

// ExtractCodeFromURL extracts the code_autorisation query parameter from a callback URL.
func ExtractCodeFromURL(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	code := u.Query().Get("code_autorisation")
	if code == "" {
		return "", fmt.Errorf("no code_autorisation found in URL: %s", rawURL)
	}
	return code, nil
}

// WaitForCallback starts a local HTTP server and blocks until the OAuth callback arrives.
// It returns the authorization code from the callback.
func WaitForCallback(port int) (string, error) {
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code_autorisation")
		if code == "" {
			http.Error(w, "Missing code_autorisation", http.StatusBadRequest)
			errCh <- fmt.Errorf("callback missing code_autorisation")
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

// ExchangeCode exchanges an OAuth authorization code for user credentials.
func ExchangeCode(ctx context.Context, code string) (*Credentials, error) {
	// Step 1: Exchange code for token
	reqURL := fmt.Sprintf(
		"%s/oauth/callback?code=%s&private_key=%s",
		defaultBaseURL,
		url.QueryEscape(code),
		url.QueryEscape(defaultPrivateKey),
	)
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create exchange request: %w", err)
	}
	req.Header.Set("X-App-Id", defaultAppID)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token exchange request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("token exchange failed: status %d", resp.StatusCode)
	}

	var tokenResp struct {
		Token  string `json:"token"`
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	// Step 2: Validate token and get profile
	loginReq, err := http.NewRequestWithContext(
		ctx, "POST", defaultBaseURL+"/user/login", strings.NewReader("extra=partner"),
	)
	if err != nil {
		return nil, fmt.Errorf("create login request: %w", err)
	}
	loginReq.Header.Set("X-App-Id", defaultAppID)
	loginReq.Header.Set("X-User-Auth-Token", tokenResp.Token)
	loginReq.Header.Set("Content-Type", "text/plain;charset=UTF-8")

	loginResp, err := http.DefaultClient.Do(loginReq)
	if err != nil {
		return nil, fmt.Errorf("login validation failed: %w", err)
	}
	defer loginResp.Body.Close()

	if loginResp.StatusCode != 200 {
		return nil, fmt.Errorf("login validation failed: status %d", loginResp.StatusCode)
	}

	var profile struct {
		User struct {
			DisplayName string `json:"display_name"`
		} `json:"user"`
	}
	json.NewDecoder(loginResp.Body).Decode(&profile)

	return &Credentials{
		AppID:         defaultAppID,
		UserAuthToken: tokenResp.Token,
		UserID:        tokenResp.UserID,
		DisplayName:   profile.User.DisplayName,
	}, nil
}

// CredentialsPath returns the default credentials file path.
func CredentialsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "qobuz", "credentials.json")
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

// NewClientFromCredentials creates a Client from saved credentials.
func NewClientFromCredentials(opts ...Option) (*Client, error) {
	creds := LoadCredentials()
	if creds == nil {
		return nil, fmt.Errorf("no credentials found at %s — run: qobuz login", CredentialsPath())
	}
	return NewClient(creds.AppID, creds.UserAuthToken, opts...), nil
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
