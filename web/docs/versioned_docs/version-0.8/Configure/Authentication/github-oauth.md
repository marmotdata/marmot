---
title: GitHub OAuth
description: Configure GitHub as an authentication provider
---

# GitHub OAuth

Marmot supports GitHub as an OAuth provider for Single Sign-On authentication.

## Create a GitHub OAuth App

1. Log in to GitHub
2. Navigate to **Settings** → **Developer settings** → **OAuth Apps**
   - Personal accounts: [https://github.com/settings/developers](https://github.com/settings/developers)
   - Organisations: `https://github.com/organisations/YOUR-ORG/settings/applications`
3. Click **New OAuth App**
4. Fill in the application details:
   - **Application name**: `Marmot`
   - **Homepage URL**: `https://your-marmot-domain.com`
   - **Authorisation callback URL**: `https://your-marmot-domain.com/auth/github/callback`
5. Click **Register application**

## Generate Client Secret

1. On the application page, click **Generate a new client secret**
2. Copy the client secret immediately

Note the **Client ID** and **Client Secret** from the application page.

## Configure Marmot

Set the following environment variables:

```bash
export MARMOT_AUTH_GITHUB_ENABLED=true
export MARMOT_AUTH_GITHUB_CLIENT_ID="your-client-id"
export MARMOT_AUTH_GITHUB_CLIENT_SECRET="your-client-secret"
```

Or configure via `config.yaml`:

```yaml
auth:
  github:
    enabled: true
    client_id: "your-client-id"
    client_secret: "your-client-secret"
```

Restart Marmot and the GitHub login button will appear on the login page.
