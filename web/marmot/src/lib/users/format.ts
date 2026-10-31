import { m } from '$lib/paraglide/messages';
import type { User } from './types';

const PROVIDERS: Record<string, string> = {
	google: 'Google',
	github: 'GitHub',
	gitlab: 'GitLab',
	okta: 'Okta',
	slack: 'Slack',
	auth0: 'Auth0',
	oidc: 'OIDC',
	saml: 'SAML'
};

/** A sign-in provider's own name, as people know it. */
export function providerName(provider: string): string {
	return PROVIDERS[provider] ?? provider.charAt(0).toUpperCase() + provider.slice(1);
}

/** Built-in roles are stored in lower case ("admin"), so the first letter is raised for display. */
export function roleLabel(name: string): string {
	return name.charAt(0).toUpperCase() + name.slice(1);
}

/** How a member signs in: each single sign-on provider they have used, or a password when they have none. */
export function signInMethods(user: Pick<User, 'identities'>): string {
	const providers = [...new Set((user.identities ?? []).map((i) => providerName(i.provider)))];
	return providers.length ? providers.join(', ') : m.users_auth_password_badge();
}
