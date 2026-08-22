# Examples

Runnable scripts for the Marmot Python SDK. Each is self-contained and prints what
it does, so they double as a smoke test against a local server.

## Prerequisites

```bash
cd sdk/python
uv sync --all-extras                 # or: pip install "marmot-sdk[claude-agent,langchain]"
export MARMOT_HOST=http://localhost:8080
export MARMOT_API_KEY=...            # or run `marmot login`, or use workload identity
```

Run them from `sdk/python`:

```bash
uv run python examples/authenticated_client.py
```

Without credentials every example fails the same way — an `AuthError` listing each
link of the chain and why it declined. That output is the fastest way to see how
credential resolution works.

## The client

| Example | What it shows |
| --- | --- |
| [`authenticated_client.py`](authenticated_client.py) | The credential chain via `connect()`, one client shared across `UsersApi`/`SearchApi`/`MetricsApi`, building a client from a credential you already hold, and typed errors (`NotFoundError`, `MarmotError`). |
| [`async_client.py`](async_client.py) | Every operation exists twice: `get_users_me()` is a coroutine, `get_users_me_sync()` runs it on a shared loop. Concurrent calls over one client with `asyncio.gather`, and `async with` to release the connection pool. |

## Workload identity

No API key to distribute: the platform attests the workload, the SDK exchanges
that attestation at `/oauth/token` (RFC 8693) for a Marmot JWT, and re-exchanges
when a request comes back `401`.

| Example | What it shows |
| --- | --- |
| [`workload_identity.py`](workload_identity.py) | The built-in sources — Kubernetes service account, GitHub Actions, GCP — used explicitly and through `connect(sources=[...])`. |
| [`custom_workload_identity.py`](custom_workload_identity/custom_workload_identity.py) | Implementing the `WorkloadIdentitySource` protocol for anywhere else, and `register_source` so `connect()` finds it. Reads an ID token from a file. |
| [`refresh_on_401.py`](custom_workload_identity/refresh_on_401.py) | A rejected token is re-exchanged and the call retried once, while a static API key surfaces the `401` instead. |
| [`local_oidc_issuer.py`](custom_workload_identity/local_oidc_issuer.py) | Not an SDK example: a throwaway OIDC issuer so the exchange can be exercised on a laptop. See below. |

### Exercising the exchange locally

The server verifies a subject token against a configured OIDC provider's JWKS, so
validating the flow needs an issuer it trusts. `local_oidc_issuer.py` serves
discovery plus a JWKS and mints one ID token to a file.

Start the issuer first — the provider performs OIDC discovery when the server
boots:

```bash
# terminal 1
uv run python examples/custom_workload_identity/local_oidc_issuer.py \
    --audience marmot-workload --email dev@example.com
```

Restart Marmot trusting it. `allowed_audiences` has no environment binding, but
the exchange falls back to `client_id`, so no config file edit is needed:

```bash
# terminal 2, from the repo root
MARMOT_AUTH_GENERIC_OIDC_ENABLED=true \
MARMOT_AUTH_GENERIC_OIDC_TYPE=generic_oidc \
MARMOT_AUTH_GENERIC_OIDC_NAME="Local Dev" \
MARMOT_AUTH_GENERIC_OIDC_URL=http://localhost:9001 \
MARMOT_AUTH_GENERIC_OIDC_CLIENT_ID=marmot-workload \
MARMOT_AUTH_GENERIC_OIDC_CLIENT_SECRET=unused \
make dev
```

Then exchange the minted token:

```bash
# terminal 3
MARMOT_HOST=http://localhost:8080 MARMOT_SUBJECT_TOKEN_FILE=/tmp/marmot-subject.jwt \
    uv run python examples/custom_workload_identity/custom_workload_identity.py
```

The same setup drives the refresh path, which replaces the client's token with a
stale one so the server answers `401` — a Marmot JWT lasts 24 hours, so waiting
for a real expiry is impractical:

```bash
MARMOT_HOST=http://localhost:8080 MARMOT_SUBJECT_TOKEN_FILE=/tmp/marmot-subject.jwt \
    uv run python examples/custom_workload_identity/refresh_on_401.py
```

Notes:

- `client_secret` must be non-empty or the provider is skipped with
  "Incomplete Generic OIDC configuration"; the exchange itself never uses it.
- A provider that fails discovery is logged, not fatal — check the server log for
  "Failed to initialize Generic OIDC provider" rather than expecting a crash.
- The token's `iss` must equal the configured `url` exactly, and its `aud` must
  match `client_id`.
- The first successful exchange **creates a user** from the token's `email`
  claim. Point the server at a throwaway database if you would rather not have
  `dev@example.com` in your dev data.
- Rejection looks like
  `{"error":"invalid_grant","error_description":"No configured OIDC provider could verify the token"}`
  — that means the token never matched a provider, not that the SDK misbehaved.
  `curl http://localhost:8080/auth-providers` shows what the server has loaded.

## Agent integrations

| Example | What it shows |
| --- | --- |
| [`claude_agent_marmot.py`](claude_agent_marmot.py) | Claude Agent SDK against Marmot's MCP endpoint, with `MarmotAgentTracker` registering the agent and recording tool calls, lineage and per-run telemetry. |
| [`langchain_marmot.py`](langchain_marmot.py) | The LangChain callback handler over a scripted chat model, so the whole callback path runs without an LLM provider. |

Both set a system prompt telling the model to consult the Marmot tools, and the
Claude example sets `setting_sources=[]`. Without that the agent inherits the
developer's own `CLAUDE.md` and answers catalog questions from it instead of
calling a tool.

## Keeping them working

`make sdk-py-lint` runs `ruff`, `ruff format --check` and `mypy` over this
directory, so the examples are type-checked against the SDK and break the build
if the API moves under them.
