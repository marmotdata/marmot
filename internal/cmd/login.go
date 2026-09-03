package cmd

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
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

var (
	loginForce           bool
	loginPrintToken      bool
	loginNoLaunchBrowser bool
)

var loginCmd = &cobra.Command{
	Use:   "login [url]",
	Short: "Authenticate with a Marmot instance via browser",
	Long: `Authenticate with a Marmot instance.

A valid cached token is reused. Otherwise a browser opens to sign in and the
token that comes back is cached under the instance's context. The token
carries the user's roles and permissions and expires after 24 hours.

The token is also registered with the Docker credential store, so oras,
crane and docker can push plugins to the registry the instance serves.`,
	Example: `  marmot login https://marmot.example.com
  marmot login marmot.example.com --force
  marmot login marmot.example.com --no-launch-browser
  crane auth login marmot.example.com -u oauth2accesstoken -p "$(marmot login marmot.example.com --print-token)"`,
	Args: cobra.MaximumNArgs(1),
	RunE: runLogin,
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove cached authentication token",
	RunE:  runLogout,
}

func init() {
	loginCmd.Flags().BoolVar(&loginForce, "force", false, "Sign in again even if a valid token is cached")
	loginCmd.Flags().BoolVar(&loginPrintToken, "print-token", false, "Print the access token on stdout (status messages go to stderr)")
	loginCmd.Flags().BoolVar(&loginNoLaunchBrowser, "no-launch-browser", false, "Print the sign-in URL instead of opening a browser")
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

	// Status goes to stderr so --print-token leaves only the token on stdout.
	status := cmd.ErrOrStderr()

	entry, ok := getCachedTokenEntry(contextName)
	if ok && !loginForce {
		fmt.Fprintf(status, "Already logged in to %s (token expires %s). Use --force to sign in again.\n",
			contextName, formatExpiry(entry.ExpiresAt))
	} else {
		tok, err := browserLogin(host, contextName, status, cmd.InOrStdin(), !loginNoLaunchBrowser)
		if err != nil {
			return err
		}
		if err := setCachedToken(contextName, tok.AccessToken, tok.TokenType, tok.ExpiresIn); err != nil {
			return fmt.Errorf("saving token: %w", err)
		}
		entry = newTokenEntry(tok.AccessToken, tok.TokenType, tok.ExpiresIn)
		fmt.Fprintf(status, "Logged in to %s (token expires %s).\n", contextName, formatExpiry(entry.ExpiresAt))
	}

	if line := describeIdentity(cmd.Context()); line != "" {
		fmt.Fprintln(status, line)
	}

	reg, err := configureRegistryAuth(host, entry.AccessToken)
	if err != nil {
		fmt.Fprintf(status, "Warning: could not register the token with the Docker credential store: %v\n", err)
	} else {
		fmt.Fprint(status, reg.describe())
	}
	fmt.Fprintf(status, "Context %q is active.\n", contextName)

	if loginPrintToken {
		fmt.Fprintln(cmd.OutOrStdout(), entry.AccessToken)
	}
	return nil
}

// browserLogin runs the OAuth authorization code flow with PKCE. The code
// arrives on a loopback listener, or pasted on stdin as the URL the browser
// landed on when it runs on another machine.
func browserLogin(host, contextName string, status io.Writer, in io.Reader, launchBrowser bool) (*tokenResponse, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("starting local server: %w", err)
	}
	defer func() { _ = listener.Close() }()

	port := listener.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://localhost:%d/callback", port)

	clientID, err := registerClient(host, redirectURI)
	if err != nil {
		return nil, err
	}

	verifier, challenge, err := generatePKCE()
	if err != nil {
		return nil, fmt.Errorf("generating PKCE: %w", err)
	}

	state, err := generateState()
	if err != nil {
		return nil, fmt.Errorf("generating state: %w", err)
	}

	authURL := fmt.Sprintf("%s/oauth/authorize?response_type=code&client_id=%s&redirect_uri=%s&state=%s&code_challenge=%s&code_challenge_method=S256&scope=openid",
		host,
		url.QueryEscape(clientID),
		url.QueryEscape(redirectURI),
		url.QueryEscape(state),
		url.QueryEscape(challenge),
	)

	// Two producers, the listener and stdin, so neither blocks after a result.
	resultCh := make(chan callbackResult, 2)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		result := parseCallback(r.URL.Query())
		if result.Err != "" {
			writeCallbackPage(w, callbackPageData{
				Title:   "Authentication failed",
				Message: "Something went wrong during sign-in. You can close this window and try again.",
				IsError: true,
			})
		} else {
			writeCallbackPage(w, callbackPageData{
				Title:   "Authentication complete",
				Message: "You're now signed in. You can close this window.",
				IsError: false,
			})
		}
		resultCh <- result
	})

	server := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = server.Serve(listener) }()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()

	opened := false
	if launchBrowser {
		fmt.Fprintf(status, "Opening browser to authenticate with %s...\n", contextName)
		opened = openBrowser(authURL) == nil
	}
	if !opened {
		fmt.Fprintf(status, "Open this URL in a browser to sign in to %s:\n\n%s\n\n", contextName, authURL)
		fmt.Fprintln(status, "If the browser runs on another machine, paste the URL it lands on here and press enter.")
	}
	go readPastedCallback(in, resultCh)

	select {
	case result := <-resultCh:
		if result.Err != "" {
			return nil, fmt.Errorf("authentication failed: %s", result.Err)
		}
		if result.State != state {
			return nil, fmt.Errorf("state mismatch, possible CSRF attack")
		}
		if result.Code == "" {
			return nil, fmt.Errorf("no authorization code received")
		}
		tok, err := exchangeCode(host, clientID, result.Code, redirectURI, verifier)
		if err != nil {
			return nil, fmt.Errorf("token exchange failed: %w", err)
		}
		return tok, nil

	case <-time.After(5 * time.Minute):
		return nil, fmt.Errorf("timed out waiting for authentication (5 minutes)")
	}
}

// parseCallback reads the code and state, or the error, from callback query
// parameters.
func parseCallback(q url.Values) callbackResult {
	if errStr := q.Get("error"); errStr != "" {
		return callbackResult{Err: fmt.Sprintf("%s: %s", errStr, q.Get("error_description"))}
	}
	return callbackResult{Code: q.Get("code"), State: q.Get("state")}
}

// readPastedCallback waits for one line on stdin and treats it as the URL the
// browser landed on. Empty input and EOF are ignored.
func readPastedCallback(in io.Reader, resultCh chan<- callbackResult) {
	line, _ := bufio.NewReader(in).ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	u, err := url.Parse(line)
	if err != nil {
		resultCh <- callbackResult{Err: fmt.Sprintf("pasted text is not a URL: %v", err)}
		return
	}
	resultCh <- parseCallback(u.Query())
}

// describeIdentity asks the server who the token belongs to. Login has
// already succeeded, so a failure here only costs the line.
func describeIdentity(ctx context.Context) string {
	c, err := newClient()
	if err != nil {
		return ""
	}
	u, err := c.Users.Me(ctx)
	if err != nil {
		return ""
	}
	roles := make([]string, 0, len(u.Roles))
	for _, r := range u.Roles {
		roles = append(roles, r.Name)
	}
	if len(roles) == 0 {
		return fmt.Sprintf("Signed in as %s.", u.Username)
	}
	return fmt.Sprintf("Signed in as %s with roles: %s.", u.Username, strings.Join(roles, ", "))
}

func formatExpiry(t time.Time) string {
	return t.UTC().Format("2006-01-02 15:04 UTC")
}

func runLogout(cmd *cobra.Command, args []string) error {
	name, ctx := getActiveContext()
	if name == "" {
		return fmt.Errorf("no active context, nothing to log out from")
	}

	if err := deleteCachedToken(name); err != nil {
		return fmt.Errorf("removing token: %w", err)
	}
	if err := removeRegistryAuth(ctx.Host); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not remove the registry credential for %s: %v\n", name, err)
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "Logged out from %s\n", name)
	return nil
}

// resolveLoginHost determines the host and context name for login.
func resolveLoginHost(args []string) (host, contextName string, err error) {
	if len(args) > 0 {
		// A bare name that matches a saved context keeps that context's
		// URL, so "marmot login localhost:8080" does not turn http into https.
		if ctx, ok := getContexts()[args[0]]; ok && !strings.Contains(args[0], "://") {
			host, contextName = ctx.Host, args[0]
		} else {
			host = normalizeHost(args[0])
		}
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

// normalizeHost adds a scheme when there is none: http for the local machine,
// https for everything else.
func normalizeHost(s string) string {
	if s == "" {
		return ""
	}
	if !strings.Contains(s, "://") {
		scheme := "https://"
		if isLoopback(s) {
			scheme = "http://"
		}
		s = scheme + s
	}
	return strings.TrimRight(s, "/")
}

func isLoopback(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

// registerClient performs Dynamic Client Registration.
func registerClient(host, redirectURI string) (string, error) {
	body := fmt.Sprintf(`{"redirect_uris":[%q],"client_name":"marmot-cli","token_endpoint_auth_method":"none"}`, redirectURI)
	resp, err := http.Post(host+"/oauth/register", "application/json", strings.NewReader(body)) //nolint:gosec // host is user-provided target server
	if err != nil {
		return "", fmt.Errorf("could not connect to %s, check that Marmot is running and the address is correct", host)
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
		return "", fmt.Errorf("login failed (HTTP %d), check that Marmot is running and the address is correct", resp.StatusCode)
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
