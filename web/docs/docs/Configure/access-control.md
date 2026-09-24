---
sidebar_position: 1
title: Users, roles and access
description: Who can sign in to open source Marmot, what each role grants, how service accounts and API keys work, and how to check what a principal can actually do.
---

import { CalloutCard, DocCard, DocCardGrid } from '@site/src/components/DocCard';
import { ThemedImg } from '@site/src/components/ThemedImg';
import { TipBox } from '@site/src/components/Steps';

# Users, roles and access

Open source Marmot has three kinds of principal (users, service accounts and anonymous visitors) and one place where their reach is decided: the role they hold. A role is a named set of permissions, a permission is a `resource:action` pair such as `assets:view`, and a role applies across the whole instance.

That instance-wide scope is the thing to understand before you plan anything. In open source Marmot there is no way to say "this team reads only these three tables": whoever holds `assets:view` reads every asset in the catalog. Per-resource grants, inheritance and a policy API are [Marmot Cloud and Marmot Enterprise](../Cloud/access-control.md).

## Administration lives under Governance

Everything on this page is at **Governance → Admin**, which needs the `users:manage` permission. The tabs are Users, Teams, Roles, Service Accounts, Authentication and System.

<ThemedImg
  lightSrc="/img/oss-admin-users-light.png"
  darkSrc="/img/oss-admin-users-dark.png"
  alt="The Users tab in the Marmot admin area, listing each user with their authentication method and roles"
/>

## Signing in

Marmot ships with a single `admin` account, password `admin`, flagged to change its password at first sign-in. Change it, then create a named admin for yourself and stop using the shared one.

Everyone else signs in one of two ways.

| Method | Set up with | Notes |
| --- | --- | --- |
| Username and password | Admin → Users → Add User | Fine for a handful of people and for break-glass access. |
| Single sign-on | [`auth.*` in your config](Authentication/index.md) | Google, GitHub, GitLab, Okta, Auth0, Keycloak, Slack or any OIDC provider. |

SSO providers are read from `config.yaml` at startup, not from the database, so the Authentication tab shows what is configured rather than letting you edit it. Restart Marmot after changing a provider.

A user created by SSO at first sign-in is given the `user` role. Nothing else is automatic: if that person needs to edit the catalog, grant it afterwards.

<TipBox variant="info" title="Anonymous visitors">
Marmot can also serve readers who never sign in. See <a href="anonymous-access">anonymous access</a>, which assigns a role to unauthenticated requests.
</TipBox>

## Roles

Two roles ship with Marmot. Both are system roles: they cannot be edited or deleted.

| Role | Grants |
| --- | --- |
| `admin` | Every permission, including user, role and service account management. |
| `user` | Read-only: assets, the glossary, ingestion runs, teams and metrics. The default for a new user. |

The `user` role deliberately excludes `assets:preview`, so a standard user sees a table's schema and documentation but not its sample rows.

Anything between those two is a role you create. Admin → Roles → **Add Role** takes a name, a description and a set of permissions; expanding a role shows what it grants today.

<ThemedImg
  lightSrc="/img/oss-admin-roles-light.png"
  darkSrc="/img/oss-admin-roles-dark.png"
  alt="The Roles tab with the user role expanded to show the permissions it grants, grouped by resource"
/>

A user can hold several roles, and their permissions are the union. Nothing subtracts, so there is no way to take a permission back other than by removing the role that carries it.

## Permissions

These are every permission in open source Marmot. Build custom roles out of them.

| Permission | Allows |
| --- | --- |
| `assets:view` | See assets, their schemas, documentation and lineage. |
| `assets:preview` | Read sample rows from table assets. |
| `assets:manage` | Create, edit and delete assets. |
| `glossary:view` / `glossary:manage` | Read terms; create, edit and delete them. |
| `ingestion:view` | See pipelines and their run history. |
| `ingestion:manage` | Create, edit and delete pipelines. |
| `teams:view` / `teams:manage` | Read teams; create, edit and delete them. |
| `users:view` / `users:manage` | Read users; create, edit and delete them. |
| `roles:manage` | Create roles and change what they grant. |
| `service_accounts:view` / `service_accounts:manage` | Read service accounts; create them and issue their API keys. |
| `sso:manage` | Map identity provider groups onto Marmot teams. |
| `metrics:view` | See catalog metrics and usage analytics. |
| `agents:emit` | Record agent run telemetry. For AI agents, not people. |
| `memory:write` | Add, edit and delete memory on assets and data products. For an agent over MCP, combine it with `assets:view`, `glossary:view` and `teams:view` in a role. |

Two of these carry more than their name suggests. `roles:manage` can grant itself every other permission, so it is effectively administrative. `service_accounts:manage` can issue an API key for any account, including one more privileged than the holder.

<TipBox variant="warning" title="Ingestion credentials">
`ingestion:manage` lets someone create a pipeline and read back its configuration. Source credentials are encrypted at rest and never returned by the API, but the permission still decides who may point Marmot at a new database. Treat it as a platform-team permission.
</TipBox>

## Teams

Teams in open source Marmot express ownership, not access. A team owns assets and data products and receives their notifications. A team carries no roles, so adding someone to one changes nothing about what they can reach.

Grant roles to people. Use teams to say who is responsible.

<ThemedImg
  lightSrc="/img/oss-admin-teams-light.png"
  darkSrc="/img/oss-admin-teams-dark.png"
  alt="The Teams tab listing teams with their descriptions and where they came from"
/>

### Mirror teams from your identity provider

A team's membership can follow a group in your identity provider instead of being maintained by hand. Map a group onto a team, and on every sign-in Marmot adds the person to the teams their groups map to and removes them from the ones they no longer carry. Only memberships Marmot itself created from that provider are removed; anyone added by hand stays. Mappings are managed through the API and need the `sso:manage` permission.

```bash
curl -X POST https://marmot.example.com/api/v1/sso/team-mappings \
  -H "Authorization: Bearer $MARMOT_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{
    "provider": "okta",
    "sso_group_name": "data-platform",
    "team_id": "<team id>",
    "member_role": "member"
  }'
```

`GET`, `PUT` and `DELETE` on `/api/v1/sso/team-mappings` list, retarget and remove them. A team that an identity provider owns is marked as such and cannot have its members edited in Marmot, which keeps the directory the single source of truth.

To skip the mapping step entirely, turn on `team_sync` for the provider and Marmot creates a team for every group it sees, filtered and prefix-stripped as you configure it:

```yaml
auth:
  okta:
    team_sync:
      enabled: true
      strip_prefix: "marmot-"
      group:
        claim: groups
        filter:
          mode: prefix
          pattern: "marmot-"
```

`mode` is `none`, `prefix`, `regex` or `allowlist`; an allowlist takes a comma-separated list of group names in `pattern`. Without a filter every group in the token becomes a team, which is rarely what you want from a directory of any size.

Mapping a group to a team does not grant anything: roles still have to be given to each person. On Marmot Cloud a team is a member you can bind a role to, so a directory group becomes a unit of access rather than only a unit of ownership.

## Service accounts

A service account is a principal with no person behind it: CI, an ingestion job, an AI agent. It holds roles exactly as a user does and authenticates with an API key instead of a browser session.

<ThemedImg
  lightSrc="/img/oss-service-accounts-light.png"
  darkSrc="/img/oss-service-accounts-dark.png"
  alt="The Service Accounts tab listing three accounts with their roles and status"
/>

Open an account to change its roles and manage its keys. An account holds up to five, each with its own name and optional expiry, which is what lets you overlap an old key and a new one during a rotation.

<ThemedImg
  lightSrc="/img/oss-service-account-keys-light.png"
  darkSrc="/img/oss-service-account-keys-dark.png"
  alt="A service account showing its details, its API keys and the roles it holds"
/>

The key is shown once, at creation. Marmot stores only a hash, so a lost key is replaced rather than recovered.

The same thing from the CLI:

```bash
marmot service-accounts create --name catalog-copilot \
  --description "Answers questions in #data-help. Owned by the platform team." \
  --roles <role-id>

marmot service-accounts apikeys create \
  --service-account <id> --name production --expires-in-days 90
```

A few habits keep this manageable:

- One account per consumer. Two agents sharing a key cannot be told apart in the audit trail, or revoked separately.
- Put the owner in the description. A year from now, that is what unblocks a rotation.
- Give an ingestion identity a custom role with `assets:manage` and nothing else, rather than `admin`.
- Set an expiry. A key with no expiry outlives the reason it was created.

## API keys for people

A person can also hold API keys of their own, from **Profile → API Keys**. Those keys carry that person's roles and stop working when the account is deactivated.

<ThemedImg
  lightSrc="/img/oss-profile-light.png"
  darkSrc="/img/oss-profile-dark.png"
  alt="The profile page with personal API keys and interface preferences"
/>

Use a personal key for your own scripts. Anything that runs without you (CI, a scheduled job, an agent) belongs on a service account, so it survives your leaving and can be revoked without locking you out.

For interactive use, `marmot login` is better than a key: it runs OAuth 2.0 with PKCE in your browser and caches a 24-hour token per [context](../cli.md#contexts), so nothing long-lived sits on disk.

## Check what a principal can do

Roles are instance-wide, so the answer is always "which roles does this principal hold, and what do those roles grant". Both halves are in the admin area.

The **Users** tab lists every user with their roles in one column, and a service account's page shows the same for accounts. The **Roles** tab expands a role to show exactly which permissions it carries, grouped by resource, so you can go from a name to a capability without leaving the page.

When something is refused, the server answers `403` and names the permission it wanted. Match that string against the table above to find which role to grant.

The one question the UI cannot answer is which credential you are currently holding. `GET /api/v1/users/me` reports the principal behind any token or API key:

```bash
curl -s https://marmot.example.com/api/v1/users/me \
  -H "Authorization: Bearer $MARMOT_API_KEY" | jq '.roles'
```

`marmot users me` does the same for your CLI session.

## Locking an instance down

Marmot's default is an open, readable catalog, which is usually right: the value of a catalog is that people find things in it. If yours holds something narrower, the three levers are:

1. Leave [anonymous access](anonymous-access.md) off, so every reader signs in.
2. Keep `assets:preview` out of the roles you hand out broadly, so sample rows stay with the people who need them.
3. Do not catalog what must not be read. Marmot stores metadata, not data, but a column name can still be sensitive.

If you need one team to see one set of assets and nothing else, that is per-resource IAM, and it is a Cloud and Enterprise capability rather than something to approximate with custom roles.

<CalloutCard
  title="Per-resource access control"
  description="Bind a role to one asset, one data product or one glossary term, review access in Terraform, and check what any principal can reach."
  docId="Cloud/access-control"
  buttonText="Marmot Cloud IAM"
  icon="mdi:shield-key"
/>

## Next steps

<DocCardGrid>
  <DocCard
    title="Authentication"
    description="Configure SSO with Google, GitHub, Okta, Auth0, Keycloak or any OIDC provider"
    docId="Configure/Authentication/index"
    icon="mdi:shield-account"
  />
  <DocCard
    title="Anonymous access"
    description="Let people browse the catalog without signing in"
    docId="Configure/anonymous-access"
    icon="mdi:incognito"
  />
  <DocCard
    title="CLI"
    description="marmot login, contexts, service accounts and API keys from the terminal"
    docId="cli"
    icon="mdi:console"
  />
  <DocCard
    title="Connect an agent"
    description="Give an AI agent its own service account and scope what it can read"
    docId="MCP/index"
    icon="mdi:robot"
  />
</DocCardGrid>
