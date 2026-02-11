---
title: Keycloak OIDC
description: Configure Keycloak as an authentication provider
---

# Keycloak OIDC

Marmot supports Keycloak as an OIDC provider for Single Sign-On authentication.

## Create a Keycloak Client

1. Log in to your Keycloak Admin Console
2. Select the realm you want to use (or create a new one)
3. Navigate to **Clients** and click **Create client**
4. Configure the client:
   - **Client type**: OpenID Connect
   - **Client ID**: `marmot` (or your preferred ID)
5. On the next step, enable **Client authentication** (this makes it a confidential client)
6. Ensure **Standard flow** is checked
7. Click **Save**
8. Configure the following under **Settings**:
   - **Valid redirect URIs**: `https://your-marmot-domain.com/auth/keycloak/callback`
   - **Web origins**: `https://your-marmot-domain.com`
9. Click **Save**

After creating the client, note:
- **Client ID**: The client ID you chose
- **Client Secret**: Found on the **Credentials** tab
- **Keycloak URL**: Your Keycloak base URL (e.g., `https://keycloak.example.com`)
- **Realm**: The realm name (e.g., `master` or your custom realm)

## Configure Marmot

Set the following environment variables:

```bash
export MARMOT_AUTH_KEYCLOAK_ENABLED=true
export MARMOT_AUTH_KEYCLOAK_CLIENT_ID="marmot"
export MARMOT_AUTH_KEYCLOAK_CLIENT_SECRET="your-client-secret"
export MARMOT_AUTH_KEYCLOAK_URL="https://keycloak.example.com"
export MARMOT_AUTH_KEYCLOAK_REALM="your-realm"
```

Or configure via `config.yaml`:

```yaml
auth:
  keycloak:
    enabled: true
    client_id: "marmot"
    client_secret: "your-client-secret"
    url: "https://keycloak.example.com"
    realm: "your-realm"
```

Marmot constructs the OIDC issuer URL automatically as `{url}/realms/{realm}` and uses OIDC discovery to configure endpoints.

Restart Marmot and the Keycloak login button will appear on the login page.

## Team Synchronisation

Marmot can automatically sync users to teams based on Keycloak group memberships.

Enable team sync:

```yaml
auth:
  keycloak:
    team_sync:
      enabled: true
      strip_prefix: "/"
      group:
        claim: "groups"
        filter:
          mode: "include"
          pattern: ".*"
```

:::tip
Keycloak prefixes group names with `/` by default (e.g., `/engineering`). Use `strip_prefix: "/"` to remove this prefix so teams are created as `engineering` instead of `/engineering`.
:::

To include groups in the ID token:

1. In your Keycloak Admin Console, navigate to **Clients** and select your Marmot client
2. Go to the **Client scopes** tab
3. Click on the `marmot-dedicated` scope (or your client's dedicated scope)
4. Click **Add mapper** → **By configuration**
5. Select **Group Membership**
6. Configure the mapper:
   - **Name**: `groups`
   - **Token Claim Name**: `groups`
   - **Full group path**: Off (recommended to get simple group names)
   - **Add to ID token**: On
   - **Add to access token**: On
   - **Add to userinfo**: On
7. Click **Save**
