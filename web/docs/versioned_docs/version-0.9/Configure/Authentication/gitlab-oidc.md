---
title: GitLab OIDC
description: Configure GitLab as an authentication provider
---

# GitLab OIDC

Marmot supports GitLab (both gitlab.com and self-hosted) as an OIDC provider for Single Sign-On authentication.

## Create a GitLab Application

1. Log in to your GitLab instance
2. Navigate to **User Settings** → **Applications** (or for groups: **Settings** → **Applications**)
   - GitLab.com: [https://gitlab.com/-/profile/applications](https://gitlab.com/-/profile/applications)
   - Self-hosted: `https://your-gitlab-instance.com/-/profile/applications`
3. Click **Add new application**
4. Fill in the application details:
   - **Name**: `Marmot`
   - **Redirect URI**: `https://your-marmot-domain.com/auth/gitlab/callback`
   - **Confidential**: Check this option
   - **Scopes**: Select `openid`, `profile`, and `email`
5. Click **Save application**

After creating the application, note:

- **Application ID**: Your client ID
- **Secret**: Your client secret

## Configure Marmot

### For GitLab.com

Set the following environment variables:

```bash
export MARMOT_AUTH_GITLAB_ENABLED=true
export MARMOT_AUTH_GITLAB_CLIENT_ID="your-application-id"
export MARMOT_AUTH_GITLAB_CLIENT_SECRET="your-secret"
```

Or configure via `config.yaml`:

```yaml
auth:
  gitlab:
    enabled: true
    client_id: "your-application-id"
    client_secret: "your-secret"
```

### For Self-Hosted GitLab

If you're using a self-hosted GitLab instance, you need to specify the URL:

```bash
export MARMOT_AUTH_GITLAB_ENABLED=true
export MARMOT_AUTH_GITLAB_CLIENT_ID="your-application-id"
export MARMOT_AUTH_GITLAB_CLIENT_SECRET="your-secret"
export MARMOT_AUTH_GITLAB_URL="https://gitlab.your-company.com"
```

Or configure via `config.yaml`:

```yaml
auth:
  gitlab:
    enabled: true
    client_id: "your-application-id"
    client_secret: "your-secret"
    url: "https://gitlab.your-company.com"
```

Restart Marmot and the GitLab login button will appear on the login page.

## Custom TLS Configuration

If your self-hosted GitLab instance uses a self-signed certificate or a certificate signed by an internal CA, you can configure Marmot to trust it:

```yaml
auth:
  gitlab:
    enabled: true
    client_id: "your-application-id"
    client_secret: "your-secret"
    url: "https://gitlab.internal"
    tls:
      ca_cert_path: "/etc/ssl/certs/internal-ca.pem"
```

Or via environment variables:

```bash
export MARMOT_AUTH_GITLAB_TLS_CA_CERT_PATH="/etc/ssl/certs/internal-ca.pem"
```

To skip TLS verification entirely (not recommended for production):

```bash
export MARMOT_AUTH_GITLAB_TLS_INSECURE_SKIP_VERIFY=true
```

If your GitLab instance requires mutual TLS (mTLS), you can provide a client certificate and key:

```yaml
auth:
  gitlab:
    tls:
      ca_cert_path: "/etc/ssl/certs/internal-ca.pem"
      cert_path: "/etc/ssl/certs/client.pem"
      key_path: "/etc/ssl/private/client-key.pem"
```

| Field                      | Description                                          |
| -------------------------- | ---------------------------------------------------- |
| `tls.ca_cert_path`         | Path to a PEM-encoded CA certificate to trust        |
| `tls.cert_path`            | Path to a PEM-encoded client certificate for mTLS    |
| `tls.key_path`             | Path to the client certificate's private key         |
| `tls.insecure_skip_verify` | Skip TLS certificate verification (default: `false`) |
