# Local TOTP contract

Upstream candidate; disabled by default. The authentication logic, API and native UI belong to Marmot.

| Password change required | Enrolled | Authentication path | Result |
| --- | --- | --- | --- |
| No | No | Local password, flag on | Session |
| No | No | Local password, `required` on | Enrollment challenge; session only after confirmation |
| No | Yes | Local password, flag on | TOTP/recovery challenge; session only after verification |
| Yes | No | Local password, flag on | Password-change challenge, then session |
| Yes | No | Local password, `required` on | Password-change challenge, then enrollment challenge |
| Yes | Yes | Local password, flag on | Password-change challenge, then TOTP/recovery challenge, then session |
| Any | Any | Local password, flag off | Existing login behavior |
| Any | Any | SSO callback | Existing IdP session flow; no native TOTP step-up |
| Any | Any | API key, service account, MCP, CLI OAuth | Existing API authentication contract; no interactive password or factor mutation |

`users/auth.go` decides the local challenge after verifying the password. `core/mfa` owns encrypted enrollment, database-serialized verification, replay prevention, recovery hashes, attempts and one-use challenges. `auth.Service` continues issuing session JWTs; challenges are opaque random values, not a second JWT dialect. The existing API validator cannot accept them as sessions. `auth.Resolver` checks an exact session epoch so factor changes revoke old sessions even within the same second.

The proposed administrative route `/users/{id}/totp` conflicts with the existing `/users/apikeys/{id}` in Go's ServeMux. Use `/users/totp/reset/{id}` instead. Recovery codes are returned only after confirmation, with a replacement session, rather than before the factor is proven. These are intentional refinements of the initial plan.

Acceptance: real PostgreSQL tests cover enrollment, replay, expiry, recovery consumption, attempts, password snapshots, concurrency and administrative reset; HTTP tests cover challenge rejection by authenticated routes, password-change ordering, flag-off behavior and API key compatibility. Native UI handles setup, confirmation, one-time recovery display, login and reset; all text uses Paraglide EN/ES.

An API key cannot be exchanged for a browser session: password changes and factor mutations require a signed local session, while administrative factor reset requires a signed session with `users:manage` and a different target user. The legacy `/users/update-password` route is available only when native TOTP is off.

Runtime configuration, API details and migration caveats: [local TOTP guide](../../web/docs/docs/Configure/Authentication/totp.md).
