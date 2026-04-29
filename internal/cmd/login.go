package cmd

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login [url]",
	Short: "Authenticate with a Marmot instance via browser",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runLogin,
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove cached authentication token",
	RunE:  runLogout,
}

func init() {
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(logoutCmd)
}

// dcrClientResponse is the response from POST /oauth/register.
type dcrClientResponse struct {
	ClientID string `json:"client_id"`
}

// tokenResponse is the response from POST /oauth/token.
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// callbackResult captures the result from the OAuth callback.
type callbackResult struct {
	Code  string
	State string
	Err   string
}

func runLogin(cmd *cobra.Command, args []string) error {
	host, contextName, err := resolveLoginHost(args)
	if err != nil {
		return err
	}

	if err := setContext(contextName, ContextEntry{Host: host}); err != nil {
		return fmt.Errorf("saving context: %w", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("starting local server: %w", err)
	}
	defer func() { _ = listener.Close() }()

	port := listener.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://localhost:%d/callback", port)

	clientID, err := registerClient(host, redirectURI)
	if err != nil {
		return err
	}

	verifier, challenge, err := generatePKCE()
	if err != nil {
		return fmt.Errorf("generating PKCE: %w", err)
	}

	state, err := generateState()
	if err != nil {
		return fmt.Errorf("generating state: %w", err)
	}

	authURL := fmt.Sprintf("%s/oauth/authorize?response_type=code&client_id=%s&redirect_uri=%s&state=%s&code_challenge=%s&code_challenge_method=S256&scope=openid",
		host,
		url.QueryEscape(clientID),
		url.QueryEscape(redirectURI),
		url.QueryEscape(state),
		url.QueryEscape(challenge),
	)

	resultCh := make(chan callbackResult, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if errStr := q.Get("error"); errStr != "" {
			desc := q.Get("error_description")
			writeCallbackPage(w, callbackPageData{
				Title:   "Authentication failed",
				Message: "Something went wrong during sign-in. You can close this window and try again.",
				IsError: true,
			})
			resultCh <- callbackResult{Err: fmt.Sprintf("%s: %s", errStr, desc)}
			return
		}

		writeCallbackPage(w, callbackPageData{
			Title:   "Authentication complete",
			Message: "You're now signed in. You can close this window.",
			IsError: false,
		})
		resultCh <- callbackResult{
			Code:  q.Get("code"),
			State: q.Get("state"),
		}
	})

	server := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = server.Serve(listener) }()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()

	fmt.Printf("Opening browser to authenticate with %s...\n", contextName)
	if err := openBrowser(authURL); err != nil {
		fmt.Printf("Could not open browser. Please visit this URL manually:\n%s\n", authURL)
	}

	select {
	case result := <-resultCh:
		if result.Err != "" {
			return fmt.Errorf("authentication failed: %s", result.Err)
		}

		if result.State != state {
			return fmt.Errorf("state mismatch — possible CSRF attack")
		}

		if result.Code == "" {
			return fmt.Errorf("no authorization code received")
		}

		tok, err := exchangeCode(host, clientID, result.Code, redirectURI, verifier)
		if err != nil {
			return fmt.Errorf("token exchange failed: %w", err)
		}

		if err := setCachedToken(contextName, tok.AccessToken, tok.TokenType, tok.ExpiresIn); err != nil {
			return fmt.Errorf("saving token: %w", err)
		}

		fmt.Printf("Successfully logged in to %s\n", contextName)
		if tok.ExpiresIn > 0 {
			expiry := time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)
			fmt.Printf("Token expires at %s\n", expiry.UTC().Format("2006-01-02 15:04:05 UTC"))
		}
		fmt.Printf("Context %q created and activated.\n", contextName)
		return nil

	case <-time.After(5 * time.Minute):
		return fmt.Errorf("timed out waiting for authentication (5 minutes)")
	}
}

func runLogout(cmd *cobra.Command, args []string) error {
	name := currentContextName()
	if name == "" {
		return fmt.Errorf("no active context — nothing to log out from")
	}

	if err := deleteCachedToken(name); err != nil {
		return fmt.Errorf("removing token: %w", err)
	}

	fmt.Printf("Logged out from %s\n", name)
	return nil
}

// resolveLoginHost determines the host and context name for login.
func resolveLoginHost(args []string) (host, contextName string, err error) {
	if len(args) > 0 {
		host = normalizeHost(args[0])
	} else if name, ctx := getActiveContext(); ctx != nil {
		host = ctx.Host
		contextName = name
	}

	if host == "" {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Marmot URL: ")
		input, _ := reader.ReadString('\n')
		host = normalizeHost(strings.TrimSpace(input))
		if host == "" {
			return "", "", fmt.Errorf("no URL provided")
		}
	}

	if contextName == "" {
		u, err := url.Parse(host)
		if err != nil {
			return "", "", fmt.Errorf("invalid URL %q: %w", host, err)
		}
		contextName = u.Hostname()
		if u.Port() != "" && u.Port() != "443" && u.Port() != "80" {
			contextName = u.Host
		}
	}

	return host, contextName, nil
}

// normalizeHost ensures the URL has a scheme.
func normalizeHost(s string) string {
	if s == "" {
		return ""
	}
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		s = "https://" + s
	}
	return strings.TrimRight(s, "/")
}

// registerClient performs Dynamic Client Registration.
func registerClient(host, redirectURI string) (string, error) {
	body := fmt.Sprintf(`{"redirect_uris":[%q],"client_name":"marmot-cli","token_endpoint_auth_method":"none"}`, redirectURI)
	resp, err := http.Post(host+"/oauth/register", "application/json", strings.NewReader(body)) //nolint:gosec // host is user-provided target server
	if err != nil {
		return "", fmt.Errorf("could not connect to %s — check that Marmot is running and the address is correct", host)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		var oauthErr struct {
			Description string `json:"error_description"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&oauthErr)

		if oauthErr.Description != "" {
			return "", fmt.Errorf("login failed: %s", oauthErr.Description)
		}
		return "", fmt.Errorf("login failed (HTTP %d) — check that Marmot is running and the address is correct", resp.StatusCode)
	}

	var dcr dcrClientResponse
	if err := json.NewDecoder(resp.Body).Decode(&dcr); err != nil {
		return "", err
	}
	return dcr.ClientID, nil
}

// generatePKCE generates a PKCE code verifier and S256 challenge.
func generatePKCE() (verifier, challenge string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}

	verifier = base64.RawURLEncoding.EncodeToString(buf)

	h := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(h[:])

	return verifier, challenge, nil
}

// generateState generates a random state parameter.
func generateState() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// exchangeCode exchanges an authorization code for a token.
func exchangeCode(host, clientID, code, redirectURI, verifier string) (*tokenResponse, error) {
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {clientID},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"code_verifier": {verifier},
	}

	resp, err := http.PostForm(host+"/oauth/token", data) //nolint:gosec // host is user-provided target server
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		var oauthErr struct {
			Error       string `json:"error"`
			Description string `json:"error_description"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&oauthErr); err == nil && oauthErr.Error != "" {
			return nil, fmt.Errorf("%s: %s", oauthErr.Error, oauthErr.Description)
		}
		return nil, fmt.Errorf("token exchange failed with status %d", resp.StatusCode)
	}

	var tok tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return nil, err
	}
	return &tok, nil
}

// openBrowser opens the given URL in the default browser.
func openBrowser(u string) error {
	switch runtime.GOOS {
	case "linux":
		return exec.Command("xdg-open", u).Start()
	case "darwin":
		return exec.Command("open", u).Start()
	case "windows":
		return exec.Command("cmd", "/c", "start", u).Start()
	default:
		return fmt.Errorf("unsupported platform %s", runtime.GOOS)
	}
}
