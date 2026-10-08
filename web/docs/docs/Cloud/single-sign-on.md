---
sidebar_position: 5
title: Single sign-on
description: Connect Okta, Entra ID, Auth0, Keycloak or any OIDC provider to your instance from the Marmot Cloud console, and map provider groups onto Marmot roles.
---

import { CalloutCard } from '@site/src/components/DocCard';
import { Steps, Step, TipBox } from '@site/src/components/Steps';

# Single sign-on

Single sign-on is configured in the Marmot Cloud console at [cloud.marmotdata.io](https://cloud.marmotdata.io), on your instance's page or in the launch form when you create it. Saving the form delivers the configuration to the instance without a restart. Nothing is configured on the instance itself.

The same form maps groups from your provider's ID token onto Marmot roles, so who is an administrator is decided by your directory.

Do this before anyone but you signs in. An instance whose only account is `admin` is converted with one form. An instance with fifty local accounts has to have each one reconciled by hand.

## Requirements

From your identity provider you need an OIDC web application with a client id and a client secret. It must be a confidential client: Marmot uses the authorization code flow, and a public or single-page client fails at the token exchange.

The application also has to put group membership in the ID token. Requesting the scope and emitting the claim are usually two separate settings, and you need both.

- Okta: add a groups claim to the ID token and request the `groups` scope.
- Entra ID: emit app-role claims. They arrive in the `roles` claim.
- Auth0: add a namespaced claim in a login action.
- Keycloak: add a `groups` client scope mapper to the client.

## Setup

<Steps>
<Step title="Open the form">

In the console, open your instance and find the Single sign-on card. Turn on **Enable single sign-on**. When launching a new instance, the same fields are in the launch form.

</Step>
<Step title="Pick your identity provider">

**Identity provider** prefills the group claim and scopes that provider uses. Every field stays editable.

| Provider | Group claim | Scopes |
| --- | --- | --- |
| Okta | `groups` | `openid, profile, email, groups` |
| Microsoft Entra ID | `roles` | `openid, profile, email` |
| Auth0 | `https://marmot/groups` | `openid, profile, email` |
| Keycloak | `groups` | `openid, profile, email` |
| Logto | `roles` | `openid, profile, email, roles` |
| Other | `groups` | `openid, profile, email` |

</Step>
<Step title="Register the redirect URI">

Copy the **Redirect URI** shown in the form and register it as the redirect URI of the application in your provider. It is your instance URL followed by `/auth/oidc/callback`.

</Step>
<Step title="Enter the application's details">

**Issuer URL** is the issuer from your provider's OpenID configuration. Marmot discovers the endpoints from it.

**Client ID** and **Client secret** are shown by your provider alongside each other. The secret is write-only: once saved, the console shows dots and never returns it. If it is lost, generate a new one and paste it here.

**Scopes** are comma separated. Leave them as prefilled unless your provider needs more.

</Step>
<Step title="Decide who gets an account">

**Allow sign-up** creates a Marmot account the first time someone signs in through the provider. Turn it on when your provider already decides who reaches your internal applications. Leave it off when the tenant behind it is much broader than the people who should read the catalog; someone then creates each account first.

**Create teams from groups** creates a Marmot team for each provider group that has no team yet, and keeps membership in step. Teams own assets and data products and are what [access grants](access-control.md#members) should name.

</Step>
<Step title="Map groups to roles">

**Group claim** is the token claim carrying group membership. **Group field** is only needed when the claim returns objects rather than strings; set it to the property to read, usually `name`.

**Role mapping** grants Marmot roles to members of a provider group. One row per group, roles comma separated.

| Provider group | Marmot roles |
| --- | --- |
| `platform-engineering` | `admin` |
| `data-engineering` | `editor, ingestion.admin` |
| `analysts` | `user` |

The roles can be the coarse `admin`, `editor` and `user` or any [granular role](access-control.md#roles). A user in several groups gets the union. Users in no mapped group keep the roles they already have; a new user with no mapped group gets the instance's default role.

</Step>
<Step title="Save and check">

Save. The sign-in page picks up the provider immediately.

Sign in as an ordinary user in a private window and look at the profile page. The roles listed there are what the mappings produced. Testing as `admin` proves nothing, because that account already holds everything.

</Step>
</Steps>

<TipBox variant="warning" title="Sign-up and the default role">
A new user matching none of your mappings gets the instance's default role, which is `user`, read-only across the catalog, unless you changed it. If the catalog holds anything not everyone in your directory should read, set the default to `catalog.none` before turning sign-up on.
</TipBox>

## Notes

- Keep one local administrator as break-glass, with a long generated password in your password manager, for the day your provider is down or a mapping change locks everyone out.
- Rotate the client secret by pasting the new value and saving. The console cannot show the old one back.
- Mapping changes apply at the next sign-in. To cut someone off immediately, revoke them in your provider.
- Turning single sign-on off clears the configuration. Local accounts keep working.

## Troubleshooting

| Symptom | Cause |
| --- | --- |
| Redirect fails right after the provider | The registered redirect URI is not exactly the one shown in the form. |
| Sign-in works, every user is read-only | The groups claim is not in the token. Check the scope is requested and the claim is configured. |
| Sign-in works, the mapping matches nothing | The claim holds objects, so set the group field. Or the group name is a path like `/analysts`. |
| New users cannot sign in | Allow sign-up is off and they have no account. |

<CalloutCard
  title="Then decide who sees what"
  description="Roles from your provider are the floor. Per-resource grants are how a sensitive data product stays sensitive."
  docId="Cloud/access-control"
  buttonText="Access control"
  icon="mdi:shield-key"
/>
