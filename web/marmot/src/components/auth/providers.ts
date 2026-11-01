// Display names and logos for the sign-in providers the server can enable, in the order the sign-in page lists them
export const signInProviders: { id: string; name: string; logo: string }[] = [
	{ id: 'okta', name: 'Okta', logo: 'okta' },
	{ id: 'google', name: 'Google', logo: 'google' },
	{ id: 'github', name: 'GitHub', logo: 'github' },
	{ id: 'gitlab', name: 'GitLab', logo: 'gitlab' },
	{ id: 'keycloak', name: 'Keycloak', logo: 'keycloak' },
	{ id: 'auth0', name: 'Auth0', logo: 'auth0' },
	{ id: 'slack', name: 'Slack', logo: 'slack' },
	{ id: 'generic_oidc', name: '', logo: 'oidc' }
];

export function providerName(id: string): string {
	return signInProviders.find((p) => p.id === id)?.name ?? id;
}
