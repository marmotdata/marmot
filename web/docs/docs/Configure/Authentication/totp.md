---
title: Local two-factor authentication
description: Optional or required TOTP for local username and password accounts
---

# Local two-factor authentication

TOTP protects **local password login** with a code from an authenticator app or a one-use recovery code. It is off by default. SSO continues to follow the identity provider's MFA policy, including accounts with both a password and a linked provider. API keys, service accounts, MCP and CLI OAuth keep their existing API authentication contracts; this is not an instance-wide MFA enforcement policy. API keys cannot change passwords, mutate factors, or reset another user's factor.

```yaml
auth:
  totp:
    enabled: true
    required: false
    issuer: Marmot
```

`required` defaults to `false`. Set it to `true` together with `enabled: true` and a server encryption key to require every local password account to enroll at its next password login. The equivalent environment variable is `MARMOT_AUTH_TOTP_REQUIRED`. An unenrolled user receives a short-lived setup challenge rather than a session; existing local sessions are refused by protected API routes until enrollment. SSO sessions and API keys retain their own authentication contracts. An inconsistent configuration fails startup. Users cannot disable TOTP while this policy is active; administrators can reset a lost factor.

Environment equivalents are `MARMOT_AUTH_TOTP_ENABLED` and `MARMOT_AUTH_TOTP_ISSUER`. Configure `MARMOT_SERVER_ENCRYPTION_KEY` using `marmot generate-encryption-key` before enrollment. TOTP uses the existing XChaCha20-Poly1305 encryptor and refuses plaintext storage even when pipeline `allow_unencrypted` is enabled. Preserve that encryption key; losing it requires administrative recovery. Turning the feature flag off disables the second-factor requirement for enrolled users too.

## Enroll and recover

1. Open **Profile → Two-factor authentication** and enter your current password.
2. Scan the QR code locally with your authenticator or enter the Base32 secret manually. Setup expires after ten minutes.
3. Confirm a six-digit code. Save the ten recovery codes immediately: the server stores only bcrypt hashes and never shows them again.
4. Future password logins request a new authenticator code or an unused recovery code. A code already used for confirmation cannot be reused for login in the same time window.

Disabling the factor requires the password and a valid second factor. Replacing recovery codes also requires both and immediately invalidates all previous recovery codes. Administrators with `users:manage` can reset another user's lost factor from **Users**, using a signed session rather than an API key; they cannot reset their own factor this way. Confirming, disabling or administratively resetting a factor invalidates existing sessions. Confirmation and self-service disabling return a replacement session to the current browser so recovery codes remain visible.

SSO-only accounts cannot enroll. Secrets, verification codes and recovery codes must not be included in logs or support requests. Keep server and authenticator clocks synchronized.

Local users can change their password in **Profile** by providing the current password. Administrators can require a local-only user to change it at their next sign-in from **Users**. This invalidates current sessions and sets the existing password-change gate; it does not send email or choose a new password for the user.

Profile password and factor management require a local signed session. When TOTP is enabled, a forced password change must use the password-login challenge endpoint; the legacy authenticated `/api/v1/users/update-password` route is unavailable. A user API key cannot be converted into a browser session through either password-change route.

## API and session contract

| Operation | Endpoint |
| --- | --- |
| Local login | `POST /api/v1/users/login` |
| Complete required password change | `POST /api/v1/users/login/password` |
| Complete second factor | `POST /api/v1/users/login/totp` |
| Prepare required enrollment | `POST /api/v1/users/login/totp/setup` |
| Confirm required enrollment | `POST /api/v1/users/login/totp/confirm` |
| Own factor status | `GET /api/v1/users/totp` |
| Admin enrollment indicators | `GET /api/v1/users/totp/enrolled` |
| Prepare enrollment | `POST /api/v1/users/totp/setup` |
| Confirm enrollment | `POST /api/v1/users/totp/confirm` |
| Disable factor | `DELETE /api/v1/users/totp` |
| Replace recovery codes | `POST /api/v1/users/totp/recovery` |
| Administrative reset | `DELETE /api/v1/users/totp/reset/{id}` |
| Change own password | `POST /api/v1/users/change-password` |
| Require password change | `POST /api/v1/users/password/require-change/{id}` |

With the flag enabled, a required password change returns `requires_password_change` and `mfa_token`; an enrolled account returns `requires_totp` and `mfa_token`. With `required: true`, an unenrolled local account returns `requires_totp_enrollment` and `mfa_token`. None of these responses contains an `access_token`. The opaque 256-bit challenge is hashed in PostgreSQL, purpose-bound and consumed once. Enrollment challenges expire after ten minutes; login and password-change challenges after five. The challenge is deliberately **not a JWT**, so the normal session validator rejects it. A new login supersedes an older pending challenge for the same account. Password changes, session invalidation and factor reset invalidate pending challenges.

Send `{ "mfa_token": "…", "code": "…" }` to complete TOTP. A mandatory password change instead takes `{ "mfa_token": "…", "new_password": "…" }` and may return another challenge; clients must not assume it grants a session.

A five-minute per-account budget permits at most five failed second-factor attempts, persisted across restarts and new challenges. Repeated batches lock the account for 5 minutes, then 15 minutes, an hour and at most 24 hours; a successful factor or administrative reset clears the escalation. HTTP verification endpoints also rate-limit callers regardless of optional instance-wide rate-limit configuration. TOTP uses SHA-1, six digits, 30-second steps and ±1-step tolerance, with transactional replay prevention. Recovery consumption and challenge consumption are atomic.

## Database migration and compatibility

The feature adds the standard PostgreSQL migration `000054_user_totp.sql`. It creates the factor, recovery-code and one-use challenge tables. Existing accounts are unchanged until TOTP is enabled.

The normal JWT now includes the exact session revocation epoch. Legacy tokens without that claim remain valid until revoked; if their second-resolution issuance time overlaps a revocation, they are rejected conservatively. SSO callbacks themselves are unchanged.
