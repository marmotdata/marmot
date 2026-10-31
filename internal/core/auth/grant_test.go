package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/marmotdata/marmot/internal/core/user"
)

type fakeGrants struct {
	valid map[string]bool
	err   error
}

func (f *fakeGrants) GrantValid(_ context.Context, userID, grantID string) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return f.valid[userID+"/"+grantID], nil
}

// A token issued to an OAuth client must stop working as soon as the person revokes the client, without waiting for the token to expire, and a failed check must refuse rather than let it through.
func TestValidateToken_ClientTokenFollowsItsGrant(t *testing.T) {
	ctx := context.Background()
	grants := &fakeGrants{valid: map[string]bool{"u1/g1": true}}
	svc := NewService(&mockRepo{}, nil, WithGrantChecker(grants))
	u := &user.User{ID: "u1", Username: "jane"}

	token, err := svc.GenerateTokenForClient(ctx, u, "client-1", "g1")
	if err != nil {
		t.Fatalf("GenerateTokenForClient: %v", err)
	}
	claims, err := svc.ValidateToken(ctx, token)
	if err != nil {
		t.Fatalf("ValidateToken with a live grant: %v", err)
	}
	if claims.ClientID != "client-1" || claims.GrantID != "g1" {
		t.Fatalf("claims carry client %q grant %q", claims.ClientID, claims.GrantID)
	}

	delete(grants.valid, "u1/g1")
	if _, err := svc.ValidateToken(ctx, token); !errors.Is(err, ErrGrantRevoked) {
		t.Fatalf("ValidateToken after revoke: err = %v, want ErrGrantRevoked", err)
	}

	grants.err = errors.New("database down")
	if _, err := svc.ValidateToken(ctx, token); err == nil {
		t.Fatal("ValidateToken accepted a client token when the grant could not be checked")
	}

	// Ordinary sign-in tokens carry no grant and never pay for the lookup.
	plain, err := svc.GenerateToken(ctx, u, nil)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	if _, err := svc.ValidateToken(ctx, plain); err != nil {
		t.Fatalf("ValidateToken for a plain token while grants fail: %v", err)
	}
}
