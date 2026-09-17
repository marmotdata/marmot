package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/marmotdata/marmot/internal/core/user"
	marmotOAuth2 "github.com/marmotdata/marmot/internal/oauth2"
	"github.com/marmotdata/marmot/pkg/config"
)

var handoffUser = &user.User{ID: "u-handoff", Username: "alice", Active: true}

func newLoginHandoffHandlerOn(repo marmotOAuth2.Repository) *Handler {
	return &Handler{
		oauthProvider: marmotOAuth2.NewProvider([]byte("test-secret-key-for-handoff---!"), repo),
		userService: &mockUserService{
			getFn: func(_ context.Context, id string) (*user.User, error) {
				if id == handoffUser.ID {
					return handoffUser, nil
				}
				return nil, user.ErrUserNotFound
			},
		},
		authService: &mockAuthService{
			generateTokenFn: func(_ context.Context, u *user.User, _ map[string]interface{}) (string, error) {
				return "marmot-jwt-for-" + u.ID, nil
			},
		},
		config: &config.Config{},
	}
}

func newLoginHandoffHandler() *Handler {
	return newLoginHandoffHandlerOn(marmotOAuth2.NewMemoryRepository())
}

func TestLoginHandoff_Issue_SetsCookie(t *testing.T) {
	h := newLoginHandoffHandler()
	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback", nil)
	rec := httptest.NewRecorder()

	if err := h.issueLoginHandoff(rec, req, handoffUser.ID); err != nil {
		t.Fatalf("issueLoginHandoff: %v", err)
	}

	res := rec.Result()
	defer res.Body.Close()

	var found *http.Cookie
	for _, c := range res.Cookies() {
		if c.Name == loginHandoffCookieName {
			found = c
			break
		}
	}
	if found == nil {
		t.Fatal("expected login handoff cookie to be set")
	}
	if !found.HttpOnly {
		t.Error("expected HttpOnly cookie")
	}
	if found.SameSite != http.SameSiteLaxMode {
		t.Errorf("expected SameSite=Lax, got %v", found.SameSite)
	}
	if found.Path != "/" {
		t.Errorf("expected Path=/, got %q", found.Path)
	}
	if found.Value == "" {
		t.Error("expected non-empty ticket value")
	}
}

// The ticket must point at a user, never at a token: nothing that grants
// access should sit in the database. The ticket itself is a credential too, so
// the row is keyed by its digest rather than by the ticket.
func TestLoginHandoff_StoresNoToken(t *testing.T) {
	repo := marmotOAuth2.NewMemoryRepository()
	h := newLoginHandoffHandlerOn(repo)

	rec := httptest.NewRecorder()
	if err := h.issueLoginHandoff(rec, httptest.NewRequest(http.MethodGet, "/", nil), handoffUser.ID); err != nil {
		t.Fatalf("issue: %v", err)
	}
	ticket := rec.Result().Cookies()[0].Value

	if _, err := repo.GetSession(context.Background(), marmotOAuth2.KindLoginHandoff, ticket); err == nil {
		t.Fatal("the raw ticket works as a key, so it was stored in the clear")
	}

	sum := sha256.Sum256([]byte(ticket))
	data, err := repo.GetSession(context.Background(), marmotOAuth2.KindLoginHandoff, hex.EncodeToString(sum[:]))
	if err != nil {
		t.Fatalf("get session by hashed ticket: %v", err)
	}
	var stored map[string]string
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if stored["user_id"] != handoffUser.ID {
		t.Fatalf("expected the user id to be stored, got %v", stored)
	}
	if len(stored) != 1 {
		t.Fatalf("expected nothing but the user id in the row, got %v", stored)
	}
}

func TestLoginHandoff_Exchange_Success(t *testing.T) {
	h := newLoginHandoffHandler()

	issueReq := httptest.NewRequest(http.MethodGet, "/", nil)
	issueRec := httptest.NewRecorder()
	if err := h.issueLoginHandoff(issueRec, issueReq, handoffUser.ID); err != nil {
		t.Fatalf("issue: %v", err)
	}
	cookies := issueRec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("no cookies issued")
	}

	exReq := httptest.NewRequest(http.MethodPost, "/auth/exchange", nil)
	for _, c := range cookies {
		exReq.AddCookie(c)
	}
	exRec := httptest.NewRecorder()
	h.handleLoginExchange(exRec, exReq)

	if exRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", exRec.Code, exRec.Body.String())
	}
	var resp map[string]string
	if err := json.NewDecoder(exRec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["access_token"] != "marmot-jwt-for-"+handoffUser.ID {
		t.Fatalf("unexpected access_token: %q", resp["access_token"])
	}
}

// The callback and the exchange are separate requests, so a load balancer can
// serve them from different replicas.
func TestLoginHandoff_ExchangeOnAnotherReplica(t *testing.T) {
	repo := marmotOAuth2.NewMemoryRepository()
	issuing := newLoginHandoffHandlerOn(repo)
	redeeming := newLoginHandoffHandlerOn(repo)

	issueRec := httptest.NewRecorder()
	if err := issuing.issueLoginHandoff(issueRec, httptest.NewRequest(http.MethodGet, "/", nil), handoffUser.ID); err != nil {
		t.Fatalf("issue: %v", err)
	}

	exReq := httptest.NewRequest(http.MethodPost, "/auth/exchange", nil)
	for _, c := range issueRec.Result().Cookies() {
		exReq.AddCookie(c)
	}
	exRec := httptest.NewRecorder()
	redeeming.handleLoginExchange(exRec, exReq)

	if exRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on another replica, got %d: %s", exRec.Code, exRec.Body.String())
	}
}

func TestLoginHandoff_NoCookie_Unauthorized(t *testing.T) {
	h := newLoginHandoffHandler()
	req := httptest.NewRequest(http.MethodPost, "/auth/exchange", nil)
	rec := httptest.NewRecorder()

	h.handleLoginExchange(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without cookie, got %d", rec.Code)
	}
}

func TestLoginHandoff_AttackerTicketValueIgnored(t *testing.T) {
	h := newLoginHandoffHandler()
	req := httptest.NewRequest(http.MethodPost, "/auth/exchange", nil)
	req.AddCookie(&http.Cookie{Name: loginHandoffCookieName, Value: "attacker-guessed-value"})
	rec := httptest.NewRecorder()

	h.handleLoginExchange(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unknown ticket, got %d", rec.Code)
	}
}

func TestLoginHandoff_SingleUse(t *testing.T) {
	h := newLoginHandoffHandler()

	issueRec := httptest.NewRecorder()
	if err := h.issueLoginHandoff(issueRec, httptest.NewRequest(http.MethodGet, "/", nil), handoffUser.ID); err != nil {
		t.Fatalf("issue: %v", err)
	}
	cookies := issueRec.Result().Cookies()

	first := httptest.NewRequest(http.MethodPost, "/auth/exchange", nil)
	for _, c := range cookies {
		first.AddCookie(c)
	}
	firstRec := httptest.NewRecorder()
	h.handleLoginExchange(firstRec, first)
	if firstRec.Code != http.StatusOK {
		t.Fatalf("expected first redemption 200, got %d", firstRec.Code)
	}

	second := httptest.NewRequest(http.MethodPost, "/auth/exchange", nil)
	for _, c := range cookies {
		second.AddCookie(c)
	}
	secondRec := httptest.NewRecorder()
	h.handleLoginExchange(secondRec, second)
	if secondRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected second redemption 401 (single-use), got %d", secondRec.Code)
	}
}

func TestLoginHandoff_Expired(t *testing.T) {
	ctx := context.Background()
	repo := marmotOAuth2.NewMemoryRepository()
	h := newLoginHandoffHandlerOn(repo)

	// Park the ticket already past its TTL, the way one looks a minute after it
	// was issued and never redeemed.
	data, err := json.Marshal(map[string]string{"user_id": handoffUser.ID})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	sum := sha256.Sum256([]byte("ticket-1"))
	if err := repo.PutSession(ctx, marmotOAuth2.KindLoginHandoff, hex.EncodeToString(sum[:]), data, -time.Second); err != nil {
		t.Fatalf("put: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/auth/exchange", nil)
	req.AddCookie(&http.Cookie{Name: loginHandoffCookieName, Value: "ticket-1"})
	rec := httptest.NewRecorder()
	h.handleLoginExchange(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for an expired ticket, got %d", rec.Code)
	}
}
