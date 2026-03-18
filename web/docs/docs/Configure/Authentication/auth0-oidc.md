---
title: Auth0 OIDC
description: Configure Auth0 as an authentication provider
---

# Auth0 OIDC

Marmot supports Auth0 as an OIDC provider for Single Sign-On authentication.

## Create an Auth0 Application

1. Log in to your Auth0 Dashboard
2. Navigate to **Applications** → **Applications**
3. Click **Create Application**
4. Choose a name for your application (e.g., `Marmot`)
5. Select **Regular Web Applications** as the application type
6. Click **Create**
7. Navigate to the **Settings** tab
8. Configure the following:
   - **Allowed Callback URLs**: `https://your-marmot-domain.com/auth/auth0/callback`
   - **Allowed Logout URLs**: `https://your-marmot-domain.com`
   - **Allowed Web Origins**: `https://your-marmot-domain.com`
9. Click **Save Changes**

After creating the application, note:
- **Client ID**: Found on the **Settings** tab
- **Client Secret**: Found on the **Settings** tab
- **Domain**: Your Auth0 domain (e.g., `https://dev-12345.us.auth0.com`)

## Configure Marmot

Set the following environment variables:

```bash
export MARMOT_AUTH_AUTH0_ENABLED=true
export MARMOT_AUTH_AUTH0_CLIENT_ID="your-client-id"
export MARMOT_AUTH_AUTH0_CLIENT_SECRET="your-client-secret"
export MARMOT_AUTH_AUTH0_URL="https://dev-12345.us.auth0.com"
```

Or configure via `config.yaml`:

```yaml
auth:
  auth0:
    enabled: true
    client_id: "your-client-id"
    client_secret: "your-client-secret"
    url: "https://dev-12345.us.auth0.com"
```

Restart Marmot and the Auth0 login button will appear on the login page.

## Team Synchronisation

Marmot can automatically sync users to teams based on Auth0 group memberships.

Enable team sync:

```yaml
auth:
  auth0:
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

1. In your Auth0 application, navigate to **Actions** → **Flows**
2. Select **Login**
3. Click **Custom** and create a new action
4. Add the following code:

```javascript
exports.onExecutePostLogin = async (event, api) => {
  if (event.authorization) {
    api.idToken.setCustomClaim('groups', event.user.groups || []);
  }
};
```

5. Deploy the action and add it to your Login flow
6. Ensure your user has groups assigned in Auth0

Alternatively, you can add groups via **Auth0 Authorization Extension** or **User Metadata**.

## Custom TLS Configuration

If your Auth0 instance uses a self-signed certificate or a certificate signed by an internal CA (e.g. Auth0 Private Cloud), you can configure Marmot to trust it:

```yaml
auth:
  auth0:
    enabled: true
    client_id: "your-client-id"
    client_secret: "your-client-secret"
    url: "https://auth.internal"
    tls:
      ca_cert_path: "/etc/ssl/certs/internal-ca.pem"
```

Or via environment variables:

```bash
export MARMOT_AUTH_AUTH0_TLS_CA_CERT_PATH="/etc/ssl/certs/internal-ca.pem"
```

To skip TLS verification entirely (not recommended for production):

```bash
export MARMOT_AUTH_AUTH0_TLS_INSECURE_SKIP_VERIFY=true
```

If your Auth0 instance requires mutual TLS (mTLS), you can provide a client certificate and key:

```yaml
auth:
  auth0:
    tls:
      ca_cert_path: "/etc/ssl/certs/internal-ca.pem"
      cert_path: "/etc/ssl/certs/client.pem"
      key_path: "/etc/ssl/private/client-key.pem"
```

| Field | Description |
|-------|-------------|
| `tls.ca_cert_path` | Path to a PEM-encoded CA certificate to trust |
| `tls.cert_path` | Path to a PEM-encoded client certificate for mTLS |
| `tls.key_path` | Path to the client certificate's private key |
| `tls.insecure_skip_verify` | Skip TLS certificate verification (default: `false`) |
