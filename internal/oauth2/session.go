package oauth2

import (
	"time"

	"github.com/ory/fosite"
)

type MarmotSession struct {
	UserID    string
	Username  string
	ExpiresAt map[fosite.TokenType]time.Time
}

func NewMarmotSession(userID, username string) *MarmotSession {
	return &MarmotSession{
		UserID:    userID,
		Username:  username,
		ExpiresAt: make(map[fosite.TokenType]time.Time),
	}
}

func (s *MarmotSession) SetExpiresAt(key fosite.TokenType, exp time.Time) {
	if s.ExpiresAt == nil {
		s.ExpiresAt = make(map[fosite.TokenType]time.Time)
	}
	s.ExpiresAt[key] = exp
}

func (s *MarmotSession) GetExpiresAt(key fosite.TokenType) time.Time {
	if s.ExpiresAt == nil {
		return time.Time{}
	}
	return s.ExpiresAt[key]
}

func (s *MarmotSession) GetUsername() string {
	return s.Username
}

func (s *MarmotSession) GetSubject() string {
	return s.UserID
}

func (s *MarmotSession) Clone() fosite.Session {
	expiresAt := make(map[fosite.TokenType]time.Time, len(s.ExpiresAt))
	for k, v := range s.ExpiresAt {
		expiresAt[k] = v
	}
	return &MarmotSession{
		UserID:    s.UserID,
		Username:  s.Username,
		ExpiresAt: expiresAt,
	}
}
