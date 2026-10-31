import { m } from '$lib/paraglide/messages';
import { holdsAtRoot, type MyAccess } from '$lib/iam/myAccess';
import type { Crumb } from '$lib/mm';

// The single definition of what Settings contains and who sees each part. The sub-navigation, the gear in the top bar, the Overview and the command palette all read it, so a section can never be reachable from one and missing from another.

/** Every Settings section route, spelled out so links type-check against the router. */
export type SettingsHref =
	| '/settings/members'
	| '/settings/teams'
	| '/settings/service-accounts'
	| '/settings/roles'
	| '/settings/authentication'
	| '/settings/system';

export interface SettingsItem {
	id: string;
	href: SettingsHref;
	label: () => string;
	description: () => string;
	/** One short line for the Overview list. */
	summary: () => string;
	/** Words people search for that the label does not say, such as SSO for Sign-in. */
	keywords: () => string;
	icon: string;
	/** Any one of these, held over the whole catalog, shows the item. Empty means admins only. */
	anyOf: string[];
}

export interface SettingsGroup {
	id: string;
	label: () => string;
	items: SettingsItem[];
}

export const settingsGroups: SettingsGroup[] = [
	{
		id: 'access',
		label: m.settings_group_access,
		items: [
			{
				id: 'members',
				href: '/settings/members',
				label: m.settings_members,
				description: m.settings_members_description,
				summary: m.settings_members_summary,
				keywords: m.settings_members_keywords,
				icon: 'material-symbols:person-outline',
				anyOf: ['users:view', 'users:manage']
			},
			{
				id: 'teams',
				href: '/settings/teams',
				label: m.settings_teams,
				description: m.settings_teams_description,
				summary: m.settings_teams_summary,
				keywords: m.settings_teams_keywords,
				icon: 'material-symbols:groups-outline',
				anyOf: ['teams:view', 'teams:manage']
			},
			{
				id: 'service-accounts',
				href: '/settings/service-accounts',
				label: m.settings_service_accounts,
				description: m.settings_service_accounts_description,
				summary: m.settings_service_accounts_summary,
				keywords: m.settings_service_accounts_keywords,
				icon: 'material-symbols:smart-toy-outline',
				anyOf: ['service_accounts:view', 'service_accounts:manage']
			},
			{
				id: 'roles',
				href: '/settings/roles',
				label: m.settings_roles,
				description: m.settings_roles_description,
				summary: m.settings_roles_summary,
				keywords: m.settings_roles_keywords,
				icon: 'material-symbols:admin-panel-settings-outline',
				anyOf: ['roles:manage']
			}
		]
	},
	{
		id: 'security',
		label: m.settings_group_security,
		items: [
			{
				id: 'authentication',
				href: '/settings/authentication',
				label: m.settings_authentication,
				description: m.settings_authentication_description,
				summary: m.settings_authentication_summary,
				keywords: m.settings_authentication_keywords,
				icon: 'material-symbols:shield-outline',
				anyOf: ['sso:manage']
			}
		]
	},
	{
		id: 'system',
		label: m.settings_group_system,
		items: [
			{
				id: 'system',
				href: '/settings/system',
				label: m.settings_system,
				description: m.settings_system_description,
				summary: m.settings_system_summary,
				keywords: m.settings_system_keywords,
				icon: 'material-symbols:dns-outline',
				anyOf: ['access:view']
			}
		]
	}
];

/** The top-bar crumbs for a Settings page: Settings, then the given trail, the last being the page itself. */
export function settingsCrumbs(...trail: Crumb[]): Crumb[] {
	return [{ label: m.settings_title(), href: '/settings' }, ...trail];
}

/** The section a Settings path belongs to, for the sub-navigation's current item. */
export function sectionForPath(path: string): SettingsItem | undefined {
	return settingsGroups
		.flatMap((group) => group.items)
		.find((item) => path === item.href || path.startsWith(item.href + '/'));
}

/** Whether the caller may see an item. */
export function canSee(access: MyAccess, item: SettingsItem): boolean {
	if (access.admin) return true;
	return item.anyOf.some((p) => holdsAtRoot(access, p));
}

/** The groups and items the caller may see, with empty groups dropped. */
export function visibleGroups(access: MyAccess): SettingsGroup[] {
	return settingsGroups
		.map((group) => ({ ...group, items: group.items.filter((item) => canSee(access, item)) }))
		.filter((group) => group.items.length > 0);
}

/** Every item the caller may see, in navigation order. */
export function visibleItems(access: MyAccess): SettingsItem[] {
	return visibleGroups(access).flatMap((group) => group.items);
}

/** A place search can take the caller to: a Settings section, or one of their own Account pages. */
export interface SettingsDestination {
	href: SettingsHref | '/account' | '/account/subscriptions' | '/account/api-keys';
	label: string;
	group: string;
	description: string;
	icon: string;
	haystack: string;
}

/** Personal pages, searchable alongside Settings because people look for "API keys" in one place. */
const accountDestinations = (): SettingsDestination[] =>
	[
		{
			href: '/account' as const,
			label: m.account_profile(),
			description: m.account_description(),
			icon: 'material-symbols:person-outline',
			keywords: m.account_profile_keywords()
		},
		{
			href: '/account/subscriptions' as const,
			label: m.account_subscriptions(),
			description: m.account_description(),
			icon: 'material-symbols:notifications-outline',
			keywords: m.account_subscriptions_keywords()
		},
		{
			href: '/account/api-keys' as const,
			label: m.account_api_keys(),
			description: m.account_description(),
			icon: 'material-symbols:key-outline',
			keywords: m.account_api_keys_keywords()
		}
	].map((d) => ({
		...d,
		group: m.account_title(),
		haystack: `${d.label} ${d.description} ${d.keywords}`.toLowerCase()
	}));

/** Everything the caller may open from Settings search, in navigation order. */
export function destinations(access: MyAccess): SettingsDestination[] {
	const settings = visibleGroups(access).flatMap((group) =>
		group.items.map((item) => ({
			href: item.href,
			label: item.label(),
			group: group.label(),
			description: item.description(),
			icon: item.icon,
			haystack:
				`${item.label()} ${group.label()} ${item.description()} ${item.keywords()}`.toLowerCase()
		}))
	);
	return [...settings, ...accountDestinations()];
}

/** The destinations matching every word of the query, best match first: a label hit outranks a keyword hit. */
export function searchSettings(access: MyAccess, query: string): SettingsDestination[] {
	const words = query.toLowerCase().split(/\s+/).filter(Boolean);
	if (words.length === 0) return [];
	return destinations(access)
		.filter((d) => words.every((w) => d.haystack.includes(w)))
		.map((d) => ({ d, score: words.filter((w) => d.label.toLowerCase().includes(w)).length }))
		.sort((a, b) => b.score - a.score)
		.map(({ d }) => d);
}
