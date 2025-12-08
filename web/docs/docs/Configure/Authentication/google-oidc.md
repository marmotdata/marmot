---
title: Google OIDC
description: Configure Google as an authentication provider
---

# Google OIDC

Marmot supports Google as an OIDC provider for Single Sign-On authentication.

## Create a Google Cloud Project

1. Go to the [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project or select an existing one

## Configure OAuth Consent Screen

1. Navigate to **APIs & Services** → **OAuth consent screen**
2. Select **External** user type (or **Internal** if using Google Workspace)
3. Configure the consent screen:
   - **App name**: `Marmot`
   - **User support email**: Your email address
   - **Developer contact information**: Your email address
4. On the **Scopes** page, add:
   - `openid`
   - `.../auth/userinfo.email`
   - `.../auth/userinfo.profile`
5. Add test users if using External user type

## Create OAuth 2.0 Credentials

1. Navigate to **APIs & Services** → **Credentials**
2. Click **Create Credentials** → **OAuth client ID**
3. Select **Web application** as the application type
4. Configure your client:
   - **Name**: `Marmot Web Client`
   - **Authorised JavaScript origins**: `https://your-marmot-domain.com`
   - **Authorised redirect URIs**: `https://your-marmot-domain.com/auth/google/callback`
5. Click **Create**

Note the **Client ID** and **Client Secret** shown in the credentials dialogue.

## Configure Marmot

Set the following environment variables:

```bash
export MARMOT_AUTH_GOOGLE_ENABLED=true
export MARMOT_AUTH_GOOGLE_CLIENT_ID="your-client-id.apps.googleusercontent.com"
export MARMOT_AUTH_GOOGLE_CLIENT_SECRET="your-client-secret"
```

Or configure via `config.yaml`:

```yaml
auth:
  google:
    enabled: true
    client_id: "your-client-id.apps.googleusercontent.com"
    client_secret: "your-client-secret"
```

Restart Marmot and the Google login button will appear on the login page.
