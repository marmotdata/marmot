---
title: Generic OIDC
description: Configure any OIDC-compliant provider for authentication
---

# Generic OIDC

Marmot supports any OIDC-compliant identity provider for Single Sign-On authentication. Use this provider when your identity provider is not listed as a dedicated integration.

You will need a **Client ID**, **Client Secret** and **Issuer URL** from your identity provider. The redirect URI to register is `https://your-marmot-domain.com/auth/generic_oidc/callback`.

## Configure Marmot

Set the following environment variables:

```bash
export MARMOT_AUTH_GENERIC_OIDC_ENABLED=true
export MARMOT_AUTH_GENERIC_OIDC_CLIENT_ID="your-client-id"
export MARMOT_AUTH_GENERIC_OIDC_CLIENT_SECRET="your-client-secret"
export MARMOT_AUTH_GENERIC_OIDC_URL="https://idp.example.com/realms/my-org"
```

Or configure via `config.yaml`:

```yaml
auth:
  generic_oidc:
    enabled: true
    client_id: "your-client-id"
    client_secret: "your-client-secret"
    url: "https://idp.example.com/realms/my-org"
```

The `url` field is the OIDC issuer URL. Marmot appends `/.well-known/openid-configuration` to discover endpoints automatically.

Restart Marmot and the SSO login button will appear on the login page.

### Custom Display Name

By default the login button reads "Sign in with SSO". You can change this with the `name` field:

```bash
export MARMOT_AUTH_GENERIC_OIDC_NAME="Corporate Login"
```

## Team Synchronisation

Marmot can automatically sync users to teams based on group claims from your identity provider.

Enable team sync:

```yaml
auth:
  generic_oidc:
    team_sync:
      enabled: true
      strip_prefix: "/"
      group:
        claim: "groups"
        filter:
          mode: "include"
          pattern: ".*"
```

Your identity provider must include a `groups` claim (or your chosen claim name) in the ID token or userinfo response. Consult your identity provider's documentation for how to configure group claims.
