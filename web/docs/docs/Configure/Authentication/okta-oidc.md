---
title: Okta OIDC
description: Configure Okta as an authentication provider
---

# Okta OIDC

Marmot supports Okta as an OIDC provider for Single Sign-On authentication.

## Create an Okta Application

1. Log in to your Okta Admin Console
2. Navigate to **Applications** → **Applications**
3. Click **Create App Integration**
4. Select **OIDC - OpenID Connect** as the sign-in method
5. Select **Web Application** as the application type
6. Configure your application:
   - **App integration name**: `Marmot`
   - **Grant type**: Check **Authorization Code**
   - **Sign-in redirect URIs**: `https://your-marmot-domain.com/auth/okta/callback`
   - **Sign-out redirect URIs**: `https://your-marmot-domain.com`
7. Click **Save**

After creating the application, note:
- **Client ID**: Found on the **General** tab
- **Client Secret**: Found on the **General** tab
- **Okta Domain**: Your Okta organisation URL (e.g., `https://dev-12345.okta.com`)

## Configure Marmot

Set the following environment variables:

```bash
export MARMOT_AUTH_OKTA_ENABLED=true
export MARMOT_AUTH_OKTA_CLIENT_ID="your-client-id"
export MARMOT_AUTH_OKTA_CLIENT_SECRET="your-client-secret"
export MARMOT_AUTH_OKTA_URL="https://dev-12345.okta.com"
```

Or configure via `config.yaml`:

```yaml
auth:
  okta:
    enabled: true
    client_id: "your-client-id"
    client_secret: "your-client-secret"
    url: "https://dev-12345.okta.com"
```

Restart Marmot and the Okta login button will appear on the login page.

## Team Synchronisation

Marmot can automatically sync users to teams based on Okta group memberships.

Enable team sync:

```yaml
auth:
  okta:
    team_sync:
      enabled: true
      strip_prefix: "marmot-"
      group:
        claim: "groups"
        filter:
          mode: "include"
          pattern: "^marmot-.*"
```

To include groups in the ID token:

1. In your Okta application, go to **Sign On** tab
2. Click **Edit** next to **OpenID Connect ID Token**
3. Under **Groups claim type**, select **Filter**
4. Configure the filter with claim name `groups` and pattern `.*`
