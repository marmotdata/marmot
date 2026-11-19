---
sidebar_position: 5
title: Set up single sign-on
description: Connect Okta, Entra ID, Auth0, Keycloak or any OIDC provider to your instance from the Marmot Cloud console, and map provider groups onto Marmot roles.
---

import { TipBox, Steps, Step } from '@site/src/components/Steps';
import { ThemedImg } from '@site/src/components/ThemedImg';

# Set up single sign-on

Let your team sign in to your instance through your identity provider over OIDC, with directory groups deciding who gets which Marmot role. You set it up in the [Marmot Cloud console](https://cloud.marmotdata.io).

## Before you start

- Your account is on Team or above.
- Your identity provider has an OIDC web application with a client ID and a client secret.

## Connect your provider

<Steps>
<Step title="Send groups in the ID token">

Marmot maps groups to roles, so the ID token has to carry them. In most providers that takes a scope and a claim:

| Provider | What to set up |
| --- | --- |
| Okta | A groups claim on the ID token, and the `groups` scope |
| Microsoft Entra ID | App roles, which arrive in the `roles` claim |
| Auth0 | A namespaced claim added in a login action |
| Keycloak | A `groups` mapper on the client |

</Step>
<Step title="Connect it in the console">

1. In the [console](https://cloud.marmotdata.io), open your instance, find **Single sign-on** and turn on **Use single sign-on**.
2. Choose your **Identity provider**, which fills in **Group claim** and **Scopes**.
3. Register the **Redirect URI** the console shows with your provider.
4. Enter the **Issuer URL**, **Client ID** and **Client secret**. The secret isn't shown again.
5. Turn on **Allow sign-up**, so people can sign in before an account exists for them.

The Single sign-on section with the provider details entered:

<ThemedImg
  lightSrc="/img/cloud-console-sso-light.png"
  darkSrc="/img/cloud-console-sso-dark.png"
  alt="The console Single sign-on section with Use single sign-on on, the redirect URI to register and an Okta issuer URL and client ID entered"
  size="panel"
/>

</Step>
<Step title="Map groups to roles">

Under **Role mapping**, add each **Provider group** with its **Marmot roles**, comma separated. For example:

| Provider group | Marmot roles |
| --- | --- |
| `data-engineering` | `editor, ingestion.admin` |
| `analytics` | `catalog.viewer` |

Optional: turn on **Create teams from groups** to keep a Marmot team in step with each provider group.

</Step>
<Step title="Save">

Select **Save single sign-on**. The instance restarts with the new configuration.

</Step>
</Steps>

<TipBox variant="warning" title="Check the base role before you allow sign-up">
Someone new who matches none of your mappings gets the base role. Unless you've changed it, that's `user`, which reads the whole catalog. [Lower the base role to `catalog.none`](grant-access.md#set-the-base-role) first if not everyone in your directory should read it.
</TipBox>

## How role mapping behaves

- Mapped roles replace a person's roles at every sign-in. Someone in several mapped groups gets all their roles.
- Someone in no mapped group keeps their roles, or gets the base role if they're new.
- Any [role](access-control.md#roles) can be mapped, and it applies across the whole catalog.

Your instance's **Settings → Authentication** has 2 more controls. **Group mappings** map a group to a team, a role or both, and their roles only apply when the console's mapping matches none of a person's groups. To admit only people invited from **Settings → Members**, turn off **Create members on their first single sign-on**:

<ThemedImg
  lightSrc="/img/cloud-new-members-light.png"
  darkSrc="/img/cloud-new-members-dark.png"
  alt="New members in Settings, Authentication, with Create members on their first single sign-on turned on"
  size="dialog"
/>

## Check that it worked

Sign in as an ordinary user in a private window, then find them under **Settings → Members** as an administrator. Their roles there are what your mappings produced. Testing as `admin` proves nothing, because it already holds every role.

## Keep it running

- Administrators can always sign in with a password, so keep one with a long generated password for the day your provider is down.
- Mapping changes apply at each person's next sign-in. To cut someone off at once, revoke them in your provider.
- Turning off **Use single sign-on** removes the configuration from the instance.

## Troubleshooting

| What you see | What to do |
| --- | --- |
| Sign-in fails as soon as the provider redirects back | Register the **Redirect URI** exactly as the console shows it. |
| "No Marmot account exists for this user and your administrator has not enabled sign-up." | Turn on **Allow sign-up**. |
| New users get the base role and no teams | The group claim isn't in the token. Request the scope and set up the claim in your provider. |
| The claim is there but no mapping matches | If the claim holds objects, set **Group field**. Otherwise enter the group name exactly as the token carries it, including any leading `/`. |

## Next steps

- [Pipelines](pipelines.md): catalog your systems on a schedule.
- [Grant and check access](grant-access.md): mapped roles apply across the whole catalog, so restrict a sensitive data product with grants on it.
