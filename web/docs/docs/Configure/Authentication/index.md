---
title: Authentication
description: Configure authentication providers
---

# Authentication

Marmot supports multiple OAuth/OIDC providers for Single Sign-On authentication. You can enable multiple providers simultaneously, and users will see login buttons for all enabled providers.

import { DocCard, DocCardGrid, FeatureCard, FeatureGrid } from '@site/src/components/DocCard';

## Supported Providers

<DocCardGrid>
  <DocCard
    title="Google OIDC"
    description="OpenID Connect authentication with Google Workspace"
    href="/docs/Configure/Authentication/google-oidc"
    icon="mdi:google"
  />
  <DocCard
    title="GitHub OAuth"
    description="OAuth 2.0 authentication with GitHub"
    href="/docs/Configure/Authentication/github-oauth"
    icon="mdi:github"
  />
  <DocCard
    title="GitLab OIDC"
    description="OpenID Connect for GitLab SaaS and self-hosted"
    href="/docs/Configure/Authentication/gitlab-oidc"
    icon="mdi:gitlab"
  />
  <DocCard
    title="Okta OIDC"
    description="OpenID Connect with team synchronisation"
    href="/docs/Configure/Authentication/okta-oidc"
    icon="simple-icons:okta"
  />
  <DocCard
    title="Slack OIDC"
    description="OpenID Connect authentication with Slack"
    href="/docs/Configure/Authentication/slack-oidc"
    icon="mdi:slack"
  />
  <DocCard
    title="Auth0 OIDC"
    description="OpenID Connect with team synchronisation"
    href="/docs/Configure/Authentication/auth0-oidc"
    icon="simple-icons:auth0"
  />
</DocCardGrid>

## How It Works

<FeatureGrid>
  <FeatureCard
    title="Email-Based Linking"
    description="User accounts are linked by email address across all providers"
    icon="mdi:email"
  />
  <FeatureCard
    title="Multiple Providers"
    description="Enable as many providers as you need - users choose at login"
    icon="mdi:account-multiple"
  />
</FeatureGrid>

## Setup Steps

Each provider requires:

1. **Create OAuth App** - Register an application in the provider's developer console
2. **Configure Marmot** - Add credentials via environment variables or config file
3. **Restart Marmot** - Changes take effect after restart

See individual provider guides above for detailed setup instructions.
