---
title: Slack OIDC
description: Configure Slack as an authentication provider
---

# Slack OIDC

Marmot supports Slack as an OIDC provider for Single Sign-On authentication, allowing users to sign in with their Slack workspace credentials.

## Create a Slack App

1. Go to [https://api.slack.com/apps](https://api.slack.com/apps)
2. Click **Create New App**
3. Select **From scratch**
4. Enter the following details:
   - **App Name**: `Marmot`
   - **Pick a workspace to develop your app in**: Select your workspace
5. Click **Create App**

## Configure OAuth & Permissions

1. In your app's settings, navigate to **OAuth & Permissions**
2. Under **Redirect URLs**, click **Add New Redirect URL**
3. Add: `https://your-marmot-domain.com/auth/slack/callback`
4. Click **Save URLs**

## Add OAuth Scopes

1. Scroll down to **Scopes** section
2. Under **User Token Scopes**, add the following scopes:
   - `openid`
   - `profile`
   - `email`

## Get Client Credentials

1. Navigate to **Basic Information** in the sidebar
2. Under **App Credentials**, you'll find:
   - **Client ID**: Your application's client ID
   - **Client Secret**: Click **Show** to reveal your client secret

## Configure Marmot

Set the following environment variables:

```bash
export MARMOT_AUTH_SLACK_ENABLED=true
export MARMOT_AUTH_SLACK_CLIENT_ID="your-client-id"
export MARMOT_AUTH_SLACK_CLIENT_SECRET="your-client-secret"
```

Or configure via `config.yaml`:

```yaml
auth:
  slack:
    enabled: true
    client_id: "your-client-id"
    client_secret: "your-client-secret"
```

Restart Marmot and the Slack login button will appear on the login page.

## Notes

- Users must be members of the Slack workspace where the app is installed
- Email addresses from Slack will be used to match or create user accounts in Marmot
- Make sure users have verified email addresses in their Slack profiles
