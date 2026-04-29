package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/marmotdata/marmot/internal/api/v1/common"
)

const (
	loginHandoffCookieName = "marmot_login_ticket"
	loginHandoffTTL        = 60 * time.Second
)

type LoginHandoffStore struct {
	mu      sync.Mutex
	tickets map[string]loginHandoffEntry
}

type loginHandoffEntry struct {
	token     string
	expiresAt time.Time
}

func NewLoginHandoffStore() *LoginHandoffStore {
	return &LoginHandoffStore{tickets: make(map[string]loginHandoffEntry)}
}

func (s *LoginHandoffStore) Put(ticket, token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tickets[ticket] = loginHandoffEntry{
		token:     token,
		expiresAt: time.Now().Add(loginHandoffTTL),
	}
}

func (s *LoginHandoffStore) Take(ticket string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.tickets[ticket]
	if !ok {
		return ""
	}
	delete(s.tickets, ticket)
	if time.Now().After(entry.expiresAt) {
		return ""
	}
	return entry.token
}

func (s *LoginHandoffStore) StartCleanup(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.cleanup()
			}
		}
	}()
}

func (s *LoginHandoffStore) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for k, e := range s.tickets {
		if now.After(e.expiresAt) {
			delete(s.tickets, k)
		}
	}
}

func (h *Handler) issueLoginHandoff(w http.ResponseWriter, r *http.Request, token string) error {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Errorf("generating handoff ticket: %w", err)
	}
	ticket := base64.RawURLEncoding.EncodeToString(buf)

	h.loginHandoffStore.Put(ticket, token)

	isSecure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	http.SetCookie(w, &http.Cookie{
		Name:     loginHandoffCookieName,
		Value:    ticket,
		Path:     "/",
		MaxAge:   int(loginHandoffTTL.Seconds()),
		Secure:   isSecure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func (h *Handler) handleLoginExchange(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(loginHandoffCookieName)
	if err != nil || cookie.Value == "" {
		common.RespondError(w, http.StatusUnauthorized, "No login handoff in progress")
		return
	}

	token := h.loginHandoffStore.Take(cookie.Value)

	isSecure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	http.SetCookie(w, &http.Cookie{
		Name:     loginHandoffCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Secure:   isSecure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	if token == "" {
		common.RespondError(w, http.StatusUnauthorized, "Login handoff expired or already used")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]string{"access_token": token})
}
