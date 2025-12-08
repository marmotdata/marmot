---
title: Authentication
description: Configure authentication providers
---

# Authentication

Marmot supports multiple OAuth/OIDC providers for Single Sign-On authentication. You can enable multiple providers simultaneously.

## Supported Providers

- [Google OIDC](./google-oidc) - OpenID Connect authentication
- [GitHub OAuth](./github-oauth) - OAuth 2.0 authentication
- [GitLab OIDC](./gitlab-oidc) - OpenID Connect authentication (SaaS and self-hosted)
- [Okta OIDC](./okta-oidc) - OpenID Connect with team synchronisation
- [Slack OIDC](./slack-oidc) - OpenID Connect authentication
- [Auth0 OIDC](./auth0-oidc) - OpenID Connect with team synchronisation

## Configuration

Each provider requires:

1. Creating an OAuth application in the provider's console
2. Setting environment variables or config file entries
3. Restarting Marmot

Users will see login buttons for all enabled providers on the login page.

## Multiple Providers

You can enable multiple providers at the same time. Users can log in with any enabled provider. Their account is linked to their email address.
