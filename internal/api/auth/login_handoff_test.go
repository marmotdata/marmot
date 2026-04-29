package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newLoginHandoffHandler() *Handler {
	return &Handler{loginHandoffStore: NewLoginHandoffStore()}
}

func TestLoginHandoff_Issue_SetsCookie(t *testing.T) {
	h := newLoginHandoffHandler()
	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback", nil)
	rec := httptest.NewRecorder()

	if err := h.issueLoginHandoff(rec, req, "jwt-value"); err != nil {
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

func TestLoginHandoff_Exchange_Success(t *testing.T) {
	h := newLoginHandoffHandler()

	issueReq := httptest.NewRequest(http.MethodGet, "/", nil)
	issueRec := httptest.NewRecorder()
	if err := h.issueLoginHandoff(issueRec, issueReq, "the-jwt"); err != nil {
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
	if resp["access_token"] != "the-jwt" {
		t.Fatalf("expected access_token=the-jwt, got %q", resp["access_token"])
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
	if err := h.issueLoginHandoff(issueRec, httptest.NewRequest(http.MethodGet, "/", nil), "jwt-1"); err != nil {
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

func TestLoginHandoff_StoreExpiry(t *testing.T) {
	s := NewLoginHandoffStore()
	s.Put("ticket-1", "jwt-1")

	s.mu.Lock()
	entry := s.tickets["ticket-1"]
	entry.expiresAt = time.Now().Add(-1 * time.Second)
	s.tickets["ticket-1"] = entry
	s.mu.Unlock()

	if got := s.Take("ticket-1"); got != "" {
		t.Fatalf("expected expired ticket to return empty, got %q", got)
	}
}
