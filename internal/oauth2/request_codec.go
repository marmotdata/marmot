package oauth2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/ory/fosite"
)

// pendingAuthorizeFormKeys are the query parameters kept when a pending
// authorize request is stored. Fosite sanitises the code and PKCE rows itself;
// this row is the one that would otherwise hold the whole query string of an
// unauthenticated request, so it gets the same treatment. The list is fosite's
// own defaults plus what its authorize and PKCE handlers read back.
var pendingAuthorizeFormKeys = map[string]bool{
	"grant_type":            true,
	"response_type":         true,
	"scope":                 true,
	"client_id":             true,
	"redirect_uri":          true,
	"code_challenge":        true,
	"code_challenge_method": true,
	"audience":              true,
}

// storedRequest is the on-disk form of a fosite request. Fosite's own types
// hold two interfaces, Client and Session, that JSON cannot round-trip on its
// own: the client is written as an id and looked up again on read, and the
// session is always a MarmotSession.
type storedRequest struct {
	// Authorize is true for the pending authorize request, which carries the
	// extra fields below. Authorization code and PKCE sessions do not.
	Authorize bool `json:"authorize,omitempty"`

	ID                string         `json:"id"`
	RequestedAt       time.Time      `json:"requested_at"`
	ClientID          string         `json:"client_id"`
	RequestedScope    []string       `json:"requested_scope,omitempty"`
	GrantedScope      []string       `json:"granted_scope,omitempty"`
	RequestedAudience []string       `json:"requested_audience,omitempty"`
	GrantedAudience   []string       `json:"granted_audience,omitempty"`
	Form              url.Values     `json:"form,omitempty"`
	Session           *MarmotSession `json:"session,omitempty"`

	ResponseTypes        []string `json:"response_types,omitempty"`
	RedirectURI          string   `json:"redirect_uri,omitempty"`
	State                string   `json:"state,omitempty"`
	HandledResponseTypes []string `json:"handled_response_types,omitempty"`
	ResponseMode         string   `json:"response_mode,omitempty"`
	DefaultResponseMode  string   `json:"default_response_mode,omitempty"`
}

func encodeRequest(req fosite.Requester) ([]byte, error) {
	sr := storedRequest{
		ID:                req.GetID(),
		RequestedAt:       req.GetRequestedAt(),
		RequestedScope:    req.GetRequestedScopes(),
		GrantedScope:      req.GetGrantedScopes(),
		RequestedAudience: req.GetRequestedAudience(),
		GrantedAudience:   req.GetGrantedAudience(),
		Form:              req.GetRequestForm(),
	}

	if c := req.GetClient(); c != nil {
		sr.ClientID = c.GetID()
	}
	if s := req.GetSession(); s != nil {
		ms, ok := s.(*MarmotSession)
		if !ok {
			// Storing it would lose the identity and the failure would only
			// show up as a missing subject at the token endpoint.
			return nil, fmt.Errorf("oauth: cannot store session of type %T", s)
		}
		sr.Session = ms
	}

	if ar, ok := req.(fosite.AuthorizeRequester); ok {
		sr.Authorize = true
		sr.ResponseTypes = ar.GetResponseTypes()
		sr.State = ar.GetState()
		sr.ResponseMode = string(ar.GetResponseMode())
		sr.DefaultResponseMode = string(ar.GetDefaultResponseMode())
		sr.Form = filterForm(sr.Form, pendingAuthorizeFormKeys)
		if u := ar.GetRedirectURI(); u != nil {
			sr.RedirectURI = u.String()
		}
		if concrete, ok := req.(*fosite.AuthorizeRequest); ok {
			sr.HandledResponseTypes = concrete.HandledResponseTypes
		}
	}

	return json.Marshal(sr)
}

func filterForm(form url.Values, allowed map[string]bool) url.Values {
	out := make(url.Values, len(allowed))
	for k, v := range form {
		if allowed[k] {
			out[k] = v
		}
	}
	return out
}

// clientLookup resolves a client id back to a client. Decoding needs it because
// the stored request only holds the id.
type clientLookup func(ctx context.Context, id string) (fosite.Client, error)

func decodeRequest(ctx context.Context, data []byte, lookup clientLookup) (fosite.Requester, error) {
	var sr storedRequest
	if err := json.Unmarshal(data, &sr); err != nil {
		return nil, fmt.Errorf("decoding stored oauth request: %w", err)
	}

	client, err := lookup(ctx, sr.ClientID)
	if err != nil {
		return nil, err
	}

	req := fosite.Request{
		ID:                sr.ID,
		RequestedAt:       sr.RequestedAt,
		Client:            client,
		RequestedScope:    sr.RequestedScope,
		GrantedScope:      sr.GrantedScope,
		RequestedAudience: sr.RequestedAudience,
		GrantedAudience:   sr.GrantedAudience,
		Form:              sr.Form,
	}
	if req.Form == nil {
		// Fosite writes into this map when it merges requests.
		req.Form = url.Values{}
	}
	if sr.Session != nil {
		req.Session = sr.Session
	}

	if !sr.Authorize {
		return &req, nil
	}

	ar := &fosite.AuthorizeRequest{
		Request:              req,
		ResponseTypes:        sr.ResponseTypes,
		State:                sr.State,
		HandledResponseTypes: sr.HandledResponseTypes,
		ResponseMode:         fosite.ResponseModeType(sr.ResponseMode),
		DefaultResponseMode:  fosite.ResponseModeType(sr.DefaultResponseMode),
	}
	if sr.RedirectURI != "" {
		u, err := url.Parse(sr.RedirectURI)
		if err != nil {
			return nil, fmt.Errorf("decoding stored redirect_uri: %w", err)
		}
		ar.RedirectURI = u
	}
	return ar, nil
}

// storedLoginHandoff is what a login handoff ticket points at.
type storedLoginHandoff struct {
	UserID string `json:"user_id"`
}

func encodeLoginHandoff(userID string) ([]byte, error) {
	return json.Marshal(storedLoginHandoff{UserID: userID})
}

func decodeLoginHandoff(data []byte) (string, error) {
	var h storedLoginHandoff
	if err := json.Unmarshal(data, &h); err != nil {
		return "", fmt.Errorf("decoding login handoff: %w", err)
	}
	if h.UserID == "" {
		return "", errors.New("login handoff has no user")
	}
	return h.UserID, nil
}
