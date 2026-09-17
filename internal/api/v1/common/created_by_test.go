package common

import (
	"context"
	"testing"

	"github.com/marmotdata/marmot/internal/core/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ctxWithUser(u *user.User) context.Context {
	return context.WithValue(context.Background(), UserContextKey, u)
}

func TestCreatedBy_ReturnsIDForPersistedUser(t *testing.T) {
	got := CreatedBy(ctxWithUser(&user.User{ID: "u-1", Username: "alice"}))

	require.NotNil(t, got)
	assert.Equal(t, "u-1", *got)
}

func TestCreatedBy_NilForAnonymousUser(t *testing.T) {
	got := CreatedBy(ctxWithUser(GetAnonymousUser("admin")))

	assert.Nil(t, got)
}

func TestCreatedBy_NilForOperatorUser(t *testing.T) {
	got := CreatedBy(ctxWithUser(GetOperatorUser()))

	assert.Nil(t, got)
}

func TestCreatedBy_NilWithoutUser(t *testing.T) {
	got := CreatedBy(context.Background())

	assert.Nil(t, got)
}

func TestIsSyntheticUser_FalseForRealUser(t *testing.T) {
	assert.False(t, IsSyntheticUser(&user.User{ID: "u-1", Username: "alice"}))
}
