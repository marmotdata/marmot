---
title: Single sign-on
description: Sign people in to open source Marmot with Google, GitHub, GitLab, Okta, Auth0, Keycloak, Slack or any OIDC provider, and manage sign-in under Settings.
---

# Single sign-on

Marmot signs people in through your identity provider over OAuth or OIDC. You can turn on several providers at once, and people pick one on the sign-in page. Accounts are matched by email address, so the same person signing in through two providers is one member.

Providers are defined in your server configuration. Group mappings, the role new members get and whether password sign-in stays on are managed under **Settings → Authentication**.

This page is about people signing in to Marmot. AI assistants and MCP clients sign in through Marmot's own OAuth server instead; see [hosted clients](../../MCP/index.md#hosted-clients).

import { DocCard, DocCardGrid } from '@site/src/components/DocCard';

## Before you start

`server.root_url` is set to the address people use to reach Marmot, because every provider's callback URL is built from it:

```yaml
server:
  root_url: https://marmot.example.com
```

`MARMOT_SERVER_ROOT_URL` does the same.

## Set up a provider

Register an application with your provider, add its client ID and secret to your configuration and restart Marmot. The guide for your provider has the details:

<DocCardGrid>
  <DocCard title="Generic OIDC" description="Any OIDC provider, with team sync" docId="Configure/Authentication/generic-oidc" icon="mdi:shield-key-outline" />
  <DocCard title="Google" description="Google Workspace accounts" docId="Configure/Authentication/google-oidc" icon="mdi:google" />
  <DocCard title="GitHub" description="GitHub accounts over OAuth" docId="Configure/Authentication/github-oauth" icon="mdi:github" />
  <DocCard title="GitLab" description="GitLab.com or self-managed GitLab" docId="Configure/Authentication/gitlab-oidc" icon="mdi:gitlab" />
  <DocCard title="Keycloak" description="Keycloak realms, with team sync" docId="Configure/Authentication/keycloak-oidc" icon="simple-icons:keycloak" />
  <DocCard title="Okta" description="Okta, with team sync" docId="Configure/Authentication/okta-oidc" icon="simple-icons:okta" />
  <DocCard title="Slack" description="Slack workspace accounts" docId="Configure/Authentication/slack-oidc" icon="mdi:slack" />
  <DocCard title="Auth0" description="Auth0 tenants, with team sync" docId="Configure/Authentication/auth0-oidc" icon="simple-icons:auth0" />
</DocCardGrid>

## Manage sign-in in Settings

**Settings → Authentication** has three parts, and needs `sso:manage`:

- **Single sign-on providers** lists the providers in your configuration. **Turn off** hides one from the sign-in page without editing the configuration.
- **Group mappings** maps a group from your identity provider to a role, a team or both. Mappings apply at every sign-in, and a mapped role replaces the base role. Select **Add group mapping** and enter the group name exactly as the provider sends it.
- **New members** decides what happens on someone's first sign-in: whether **Create members on their first single sign-on** is on (when it's off, only invited people can sign in), the **Default role** they get, which is the base role, a **Default team** and whether **Allow password sign-in** stays on. Admins can always sign in with a password, in case the provider is down.

To create a team for every group automatically instead of mapping them one by one, turn on `team_sync` in the provider's configuration. The Okta, Auth0, Keycloak and generic OIDC guides show how.

## Next steps

- [Users, roles and access](../access-control.md): what each role grants.
- [Anonymous access](../anonymous-access.md): let people browse without signing in.
