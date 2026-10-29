package mfa

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/marmotdata/marmot/internal/crypto"
	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/require"
)

func TestEnrollmentLoginAndRecovery(t *testing.T) {
	ctx := context.Background()
	db := pgtest.TempDB(t)
	var id string
	require.NoError(t, db.QueryRow(ctx, `UPDATE users SET must_change_password=false WHERE username='admin' RETURNING id`).Scan(&id))
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	enc, err := crypto.NewEncryptor(key)
	require.NoError(t, err)
	s := NewService(db, enc, "Marmot test")
	now := time.Now().Add(time.Hour)
	s.now = func() time.Time { return now }
	initial, err := s.Status(ctx, id)
	require.NoError(t, err)
	require.True(t, initial.Local)
	require.False(t, initial.Enabled)
	_, err = NewService(db, nil, "").Setup(ctx, id)
	require.ErrorIs(t, err, ErrUnavailable)
	setup, err := s.Setup(ctx, id)
	require.NoError(t, err)
	require.Contains(t, setup.URI, "otpauth://totp/")
	require.True(t, strings.HasPrefix(setup.QR, "data:image/png;base64,"))
	var stored string
	require.NoError(t, db.QueryRow(ctx, `SELECT pending_ciphertext FROM user_totp WHERE user_id=$1`, id).Scan(&stored))
	require.NotEqual(t, setup.Secret, stored)
	code, err := totp.GenerateCode(setup.Secret, now)
	require.NoError(t, err)
	recovery, err := s.Confirm(ctx, id, code)
	require.NoError(t, err)
	require.Len(t, recovery, 10)
	st, err := s.Status(ctx, id)
	require.NoError(t, err)
	require.True(t, st.Enabled)
	require.Equal(t, 10, st.RecoveryRemaining)
	_, err = s.Setup(ctx, id)
	require.ErrorIs(t, err, ErrAlreadyEnabled)
	token, err := s.IssueChallenge(ctx, id, PurposeTOTP)
	require.NoError(t, err)
	_, err = s.VerifyLogin(ctx, token, code)
	require.ErrorIs(t, err, ErrInvalidCode) // confirmation consumed this step
	now = now.Add(30 * time.Second)
	code, err = totp.GenerateCode(setup.Secret, now)
	require.NoError(t, err)
	got, err := s.VerifyLogin(ctx, token, code)
	require.NoError(t, err)
	require.Equal(t, id, got)
	_, err = s.VerifyLogin(ctx, token, code)
	require.ErrorIs(t, err, ErrInvalidChallenge)
	token, err = s.IssueChallenge(ctx, id, PurposeTOTP)
	require.NoError(t, err)
	_, err = s.VerifyLogin(ctx, token, code)
	require.ErrorIs(t, err, ErrInvalidCode) // consumed OTP stays consumed across new challenges
	outOfWindow, err := totp.GenerateCode(setup.Secret, now.Add(90*time.Second))
	require.NoError(t, err)
	_, err = s.VerifyLogin(ctx, token, outOfWindow)
	require.ErrorIs(t, err, ErrInvalidCode)
	// Two concurrent requests cannot spend the same challenge/recovery twice.
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Go(func() { _, e := s.VerifyLogin(ctx, token, recovery[0]); results <- e })
	}
	wg.Wait()
	close(results)
	successes := 0
	for e := range results {
		if e == nil {
			successes++
		} else {
			require.ErrorIs(t, e, ErrInvalidChallenge)
		}
	}
	require.Equal(t, 1, successes)
	token, err = s.IssueChallenge(ctx, id, PurposeTOTP)
	require.NoError(t, err)
	_, err = s.VerifyLogin(ctx, token, recovery[0])
	require.ErrorIs(t, err, ErrInvalidCode)
	st, err = s.Status(ctx, id)
	require.NoError(t, err)
	require.Equal(t, 9, st.RecoveryRemaining)
	regenerated, err := s.RegenerateRecovery(ctx, id, recovery[1])
	require.NoError(t, err)
	require.Len(t, regenerated, 10)
	_, err = s.VerifyLogin(ctx, token, recovery[2])
	require.ErrorIs(t, err, ErrInvalidCode)
	require.NoError(t, s.Disable(ctx, id, regenerated[0]))
	_, err = s.VerifyLogin(ctx, token, regenerated[1])
	require.ErrorIs(t, err, ErrInvalidChallenge)
	st, err = s.Status(ctx, id)
	require.NoError(t, err)
	require.False(t, st.Enabled)
	require.Zero(t, st.RecoveryRemaining)
	var cutoff *time.Time
	require.NoError(t, db.QueryRow(ctx, `SELECT sessions_invalidated_at FROM users WHERE id=$1`, id).Scan(&cutoff))
	require.NotNil(t, cutoff)
}

func TestLimitsPendingAndChallengeAuthority(t *testing.T) {
	ctx := context.Background()
	db := pgtest.TempDB(t)
	var id string
	require.NoError(t, db.QueryRow(ctx, `UPDATE users SET must_change_password=true WHERE username='admin' RETURNING id`).Scan(&id))
	enc, err := crypto.NewEncryptor(make([]byte, 32))
	require.NoError(t, err)
	s := NewService(db, enc, "")
	now := time.Now().Add(time.Hour)
	s.now = func() time.Time { return now }
	token, err := s.IssueChallenge(ctx, id, PurposePasswordChange)
	require.NoError(t, err)
	_, err = s.VerifyLogin(ctx, token, "123456")
	require.ErrorIs(t, err, ErrInvalidChallenge)
	got, err := s.ConsumePasswordChallenge(ctx, token)
	require.NoError(t, err)
	require.Equal(t, id, got)
	_, err = s.ConsumePasswordChallenge(ctx, token)
	require.ErrorIs(t, err, ErrInvalidChallenge)
	token, err = s.IssueChallenge(ctx, id, PurposePasswordChange)
	require.NoError(t, err)
	now = now.Add(5 * time.Minute)
	_, err = s.ConsumePasswordChallenge(ctx, token)
	require.ErrorIs(t, err, ErrInvalidChallenge)
	token, err = s.IssueChallenge(ctx, id, PurposePasswordChange)
	require.NoError(t, err)
	_, err = db.Exec(ctx, `UPDATE users SET password_hash=password_hash || 'changed' WHERE id=$1`, id)
	require.NoError(t, err)
	_, err = s.ConsumePasswordChallenge(ctx, token)
	require.ErrorIs(t, err, ErrInvalidChallenge)
	token, err = s.IssueChallenge(ctx, id, PurposePasswordChange)
	require.NoError(t, err)
	_, err = db.Exec(ctx, `UPDATE users SET sessions_invalidated_at=$2 WHERE id=$1`, id, now)
	require.NoError(t, err)
	_, err = s.ConsumePasswordChallenge(ctx, token)
	require.ErrorIs(t, err, ErrInvalidChallenge)
	setup, err := s.Setup(ctx, id)
	require.NoError(t, err)
	for attempt := range 5 {
		_, err = s.Confirm(ctx, id, "wrong")
		if attempt == 4 {
			require.ErrorIs(t, err, ErrRateLimited)
		} else {
			require.ErrorIs(t, err, ErrInvalidCode)
		}
	}
	setup, err = s.Setup(ctx, id)
	require.NoError(t, err) // setup cannot clear persisted lockout
	code, err := totp.GenerateCode(setup.Secret, now)
	require.NoError(t, err)
	_, err = s.Confirm(ctx, id, code)
	require.ErrorIs(t, err, ErrRateLimited)
	now = now.Add(5 * time.Minute)
	code, err = totp.GenerateCode(setup.Secret, now)
	require.NoError(t, err)
	_, err = s.Confirm(ctx, id, code)
	require.NoError(t, err)
	_, err = db.Exec(ctx, `UPDATE users SET must_change_password=false WHERE id=$1`, id)
	require.NoError(t, err)
	for attempt := range 5 {
		token, err = s.IssueChallenge(ctx, id, PurposeTOTP)
		require.NoError(t, err)
		_, err = s.VerifyLogin(ctx, token, "wrong")
		if attempt == 4 {
			require.ErrorIs(t, err, ErrRateLimited)
		} else {
			require.ErrorIs(t, err, ErrInvalidCode)
		}
	}
	token, err = s.IssueChallenge(ctx, id, PurposeTOTP)
	require.NoError(t, err)
	_, err = s.VerifyLogin(ctx, token, code)
	require.ErrorIs(t, err, ErrRateLimited)
	now = now.Add(5 * time.Minute)
	for attempt := range 5 {
		token, err = s.IssueChallenge(ctx, id, PurposeTOTP)
		require.NoError(t, err)
		_, err = s.VerifyLogin(ctx, token, "wrong")
		if attempt == 4 {
			require.ErrorIs(t, err, ErrRateLimited)
		} else {
			require.ErrorIs(t, err, ErrInvalidCode)
		}
	}
	var level int
	var lockedUntil time.Time
	require.NoError(t, db.QueryRow(ctx, `SELECT lockout_level,locked_until FROM user_totp WHERE user_id=$1`, id).Scan(&level, &lockedUntil))
	require.Equal(t, 2, level)
	require.WithinDuration(t, now.Add(15*time.Minute), lockedUntil, time.Second)
	require.NoError(t, s.Reset(ctx, id))
	_, err = s.VerifyLogin(ctx, token, code)
	require.ErrorIs(t, err, ErrInvalidChallenge)
	setup, err = s.Setup(ctx, id)
	require.NoError(t, err)
	now = now.Add(11 * time.Minute)
	code, err = totp.GenerateCode(setup.Secret, now)
	require.NoError(t, err)
	_, err = s.Confirm(ctx, id, code)
	require.ErrorIs(t, err, ErrInvalidCode)
	_, err = db.Exec(ctx, `UPDATE users SET password_hash=NULL WHERE id=$1`, id)
	require.NoError(t, err)
	_, err = s.Setup(ctx, id)
	require.ErrorIs(t, err, ErrNotLocal)
	_, err = s.IssueChallenge(ctx, id, PurposePasswordChange)
	require.ErrorIs(t, err, ErrNotLocal)
}
